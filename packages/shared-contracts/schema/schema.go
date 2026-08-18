// Package schema exposes the repository's language-neutral JSON Schemas to
// Product API code.  The schema file is embedded from this same directory so
// production validation and the checked-in contract cannot silently drift.
package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"

	_ "embed"
)

//go:embed timeline.schema.json
var timelineSchemaFile []byte

// TimelineSchema returns a copy of the canonical TimelineVersion schema.
func TimelineSchema() []byte {
	return append([]byte(nil), timelineSchemaFile...)
}

// ValidateTimelineSchema validates the structural JSON Schema contract.  It
// deliberately supports the bounded Draft 2020-12 subset used by the shared
// contracts (including $defs, oneOf, additionalProperties and numeric/string
// bounds), while domain packages enforce cross-reference semantics separately.
func ValidateTimelineSchema(document []byte) error {
	return validateDocument(timelineSchemaFile, document)
}

func validateDocument(schemaBytes, document []byte) error {
	var schemaValue, documentValue any
	if err := decodeSingle(schemaBytes, &schemaValue); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	if err := decodeSingle(document, &documentValue); err != nil {
		return fmt.Errorf("document: %w", err)
	}
	schemaObject, ok := schemaValue.(map[string]any)
	if !ok {
		return errors.New("schema root must be an object")
	}
	if err := validateSchemaValue(schemaObject, documentValue, "timeline", schemaObject); err != nil {
		return err
	}
	return nil
}

func decodeSingle(value []byte, target *any) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return errors.New("contains trailing JSON")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func validateSchemaValue(schema map[string]any, value any, path string, root map[string]any) error {
	if ref, ok := schema["$ref"].(string); ok {
		const prefix = "#/$defs/"
		if !strings.HasPrefix(ref, prefix) {
			return fmt.Errorf("%s uses unsupported reference %q", path, ref)
		}
		defs, ok := root["$defs"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s reference table is missing", path)
		}
		definition, ok := defs[strings.TrimPrefix(ref, prefix)].(map[string]any)
		if !ok {
			return fmt.Errorf("%s references unknown definition %q", path, ref)
		}
		return validateSchemaValue(definition, value, path, root)
	}
	if raw, ok := schema["oneOf"]; ok {
		return validateComposition(raw, value, path, root, true)
	}
	if raw, ok := schema["anyOf"]; ok {
		return validateComposition(raw, value, path, root, false)
	}
	if raw, ok := schema["allOf"]; ok {
		if err := validateAllOf(raw, value, path, root); err != nil {
			return err
		}
	}
	if constant, ok := schema["const"]; ok && !reflect.DeepEqual(constant, value) {
		return fmt.Errorf("%s must equal %v", path, constant)
	}
	if raw, ok := schema["enum"]; ok {
		values, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("%s has malformed enum", path)
		}
		matched := false
		for _, candidate := range values {
			if reflect.DeepEqual(candidate, value) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%s is not an allowed value", path)
		}
	}
	if rawType, ok := schema["type"]; ok {
		if !matchesType(rawType, value) {
			return fmt.Errorf("%s has the wrong type", path)
		}
	}
	if err := validateBounds(schema, value, path); err != nil {
		return err
	}
	switch current := value.(type) {
	case map[string]any:
		return validateObject(schema, current, path, root)
	case []any:
		return validateArray(schema, current, path, root)
	case string:
		return validateString(schema, current, path)
	}
	return nil
}

func validateComposition(raw any, value any, path string, root map[string]any, exactlyOne bool) error {
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		return fmt.Errorf("%s has malformed schema composition", path)
	}
	matched := 0
	for index, candidate := range values {
		schema, ok := candidate.(map[string]any)
		if !ok {
			return fmt.Errorf("%s schema branch %d is malformed", path, index)
		}
		if validateSchemaValue(schema, value, fmt.Sprintf("%s[%d]", path, index), root) == nil {
			matched++
		}
	}
	if (exactlyOne && matched != 1) || (!exactlyOne && matched == 0) {
		return fmt.Errorf("%s matches %d schema branches", path, matched)
	}
	return nil
}

func validateAllOf(raw any, value any, path string, root map[string]any) error {
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		return fmt.Errorf("%s has malformed schema composition", path)
	}
	for index, candidate := range values {
		schema, ok := candidate.(map[string]any)
		if !ok {
			return fmt.Errorf("%s schema branch %d is malformed", path, index)
		}
		if err := validateSchemaValue(schema, value, fmt.Sprintf("%s[%d]", path, index), root); err != nil {
			return err
		}
	}
	return nil
}

func validateObject(schema map[string]any, value map[string]any, path string, root map[string]any) error {
	if raw, ok := schema["required"]; ok {
		required, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("%s has malformed required fields", path)
		}
		for _, field := range required {
			name, ok := field.(string)
			if !ok || name == "" {
				return fmt.Errorf("%s has malformed required field", path)
			}
			if _, exists := value[name]; !exists {
				return fmt.Errorf("%s.%s is required", path, name)
			}
		}
	}
	properties, _ := schema["properties"].(map[string]any)
	for name, child := range value {
		childSchema, known := properties[name].(map[string]any)
		if !known {
			additional := schema["additionalProperties"]
			if additional == false {
				return fmt.Errorf("%s.%s is not allowed", path, name)
			}
			if additionalSchema, ok := additional.(map[string]any); ok {
				if err := validateSchemaValue(additionalSchema, child, path+"."+name, root); err != nil {
					return err
				}
			}
			continue
		}
		if err := validateSchemaValue(childSchema, child, path+"."+name, root); err != nil {
			return err
		}
	}
	if maximum, ok := integerConstraint(schema, "maxProperties"); ok && len(value) > maximum {
		return fmt.Errorf("%s has too many properties", path)
	}
	if minimum, ok := integerConstraint(schema, "minProperties"); ok && len(value) < minimum {
		return fmt.Errorf("%s has too few properties", path)
	}
	return nil
}

func validateArray(schema map[string]any, value []any, path string, root map[string]any) error {
	if minimum, ok := integerConstraint(schema, "minItems"); ok && len(value) < minimum {
		return fmt.Errorf("%s has too few items", path)
	}
	if maximum, ok := integerConstraint(schema, "maxItems"); ok && len(value) > maximum {
		return fmt.Errorf("%s has too many items", path)
	}
	if unique, ok := schema["uniqueItems"].(bool); ok && unique {
		seen := map[string]bool{}
		for index, item := range value {
			encoded, _ := json.Marshal(item)
			key := string(encoded)
			if seen[key] {
				return fmt.Errorf("%s[%d] duplicates an earlier item", path, index)
			}
			seen[key] = true
		}
	}
	if itemSchema, ok := schema["items"].(map[string]any); ok {
		for index, item := range value {
			if err := validateSchemaValue(itemSchema, item, fmt.Sprintf("%s[%d]", path, index), root); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateString(schema map[string]any, value, path string) error {
	length := utf8.RuneCountInString(value)
	if minimum, ok := integerConstraint(schema, "minLength"); ok && length < minimum {
		return fmt.Errorf("%s is shorter than allowed", path)
	}
	if maximum, ok := integerConstraint(schema, "maxLength"); ok && length > maximum {
		return fmt.Errorf("%s is longer than allowed", path)
	}
	if pattern, ok := schema["pattern"].(string); ok {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("%s has an invalid schema pattern: %w", path, err)
		}
		if !compiled.MatchString(value) {
			return fmt.Errorf("%s does not match the required pattern", path)
		}
	}
	return nil
}

func validateBounds(schema map[string]any, value any, path string) error {
	number, ok := numericValue(value)
	if !ok {
		return nil
	}
	for _, name := range []string{"minimum", "exclusiveMinimum", "maximum", "exclusiveMaximum", "multipleOf"} {
		bound, exists := schema[name]
		if !exists {
			continue
		}
		boundNumber, ok := numericValue(bound)
		if !ok {
			return fmt.Errorf("%s has malformed numeric bound %s", path, name)
		}
		switch name {
		case "minimum":
			if number < boundNumber {
				return fmt.Errorf("%s is below minimum", path)
			}
		case "exclusiveMinimum":
			if number <= boundNumber {
				return fmt.Errorf("%s is at or below exclusive minimum", path)
			}
		case "maximum":
			if number > boundNumber {
				return fmt.Errorf("%s is above maximum", path)
			}
		case "exclusiveMaximum":
			if number >= boundNumber {
				return fmt.Errorf("%s is at or above exclusive maximum", path)
			}
		case "multipleOf":
			if boundNumber > 0 && math.Abs(number/boundNumber-math.Round(number/boundNumber)) > 1e-9 {
				return fmt.Errorf("%s is not a multiple of the required value", path)
			}
		}
	}
	return nil
}

func matchesType(raw any, value any) bool {
	if values, ok := raw.([]any); ok {
		for _, item := range values {
			if matchesType(item, value) {
				return true
			}
		}
		return false
	}
	name, ok := raw.(string)
	if !ok {
		return false
	}
	switch name {
	case "null":
		return value == nil
	case "boolean":
		_, ok = value.(bool)
	case "object":
		_, ok = value.(map[string]any)
	case "array":
		_, ok = value.([]any)
	case "number":
		_, ok = numericValue(value)
	case "integer":
		number, numberOK := numericValue(value)
		return numberOK && math.Trunc(number) == number
	case "string":
		_, ok = value.(string)
	default:
		return false
	}
	return ok
}

func numericValue(value any) (float64, bool) {
	switch number := value.(type) {
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)

	case float64:
		return number, !math.IsNaN(number) && !math.IsInf(number, 0)
	default:
		return 0, false
	}
}

func integerConstraint(schema map[string]any, key string) (int, bool) {
	number, ok := numericValue(schema[key])
	if !ok || number < 0 || math.Trunc(number) != number {
		return 0, false
	}
	return int(number), true
}
