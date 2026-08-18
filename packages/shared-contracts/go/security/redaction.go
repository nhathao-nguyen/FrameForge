package security

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var sensitiveKey = regexp.MustCompile(`(?i)(authorization|password|secret|token|presigned|traceback|credential|private[_-]?key)`)
var absolutePath = regexp.MustCompile(`(?i)([a-z]:[\\/]|\\\\|/var/|/home/|/tmp/|/private/)`)

func RedactText(value string) string {
	if sensitiveKey.MatchString(value) || absolutePath.MatchString(value) {
		return "[REDACTED]"
	}
	if len(value) > 512 {
		return value[:512] + "...[TRUNCATED]"
	}
	return value
}

func RedactFields(fields map[string]string) map[string]string {
	result := make(map[string]string, len(fields))
	for key, value := range fields {
		if sensitiveKey.MatchString(key) || strings.Contains(strings.ToLower(key), "path") || strings.Contains(strings.ToLower(key), "url") {
			result[key] = "[REDACTED]"
			continue
		}
		result[key] = RedactText(value)
	}
	return result
}

func SafeError(err error) string {
	if err == nil {
		return ""
	}
	return RedactText(fmt.Sprintf("%v", err))
}

func IsSafeRelativeName(name string) bool {
	return name != "" && filepath.Base(name) == name && !strings.ContainsAny(name, "\\/\r\n\x00")
}
