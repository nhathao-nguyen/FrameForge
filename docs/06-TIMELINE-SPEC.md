# 06 — Timeline Specification

## 1. Canonical role

```text
AI Matching
  → match proposal Artifact
  → build_timeline
  → Timeline proposal
  → Human Edit / automatic approval policy
  → exact TimelineVersion
  → Renderer
```

TimelineVersion là source of truth duy nhất cho placement/edit decision. Renderer không đọc `matches.json` để chọn lại Scene, không rerun AI matching và không silently thay user clip. RenderProfile thay output constraints, không sửa canonical edit decisions.

- Timebase canonical: seconds, JSON number, precision ≥ 1 ms.
- Range dùng half-open `[in, out)` và `out > in`.
- `timeline_in_sec`/`timeline_out_sec` là output placement.
- `source_in_sec`/`source_out_sec` là range trong exact source Asset/Artifact/Scene.
- IDs Track/Clip ổn định trong Timeline lineage; edit tạo TimelineVersion mới.
- Document không chứa media bytes, secret, URL hết hạn hoặc filesystem path.

## 2. JSON Schema Draft 2020-12

Canonical contract version `1.0`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "urn:nh-media:timeline:1.0",
  "title": "AI Video Production TimelineVersion",
  "type": "object",
  "additionalProperties": false,
  "required": [
    "schema_version", "timeline_id", "timeline_version_id",
    "project_id", "version", "duration_sec", "tracks"
  ],
  "properties": {
    "schema_version": {"const": "1.0"},
    "timeline_id": {"type": "string", "minLength": 1},
    "timeline_version_id": {"type": "string", "minLength": 1},
    "project_id": {"type": "string", "minLength": 1},
    "version": {"type": "integer", "minimum": 1},
    "duration_sec": {"type": "number", "exclusiveMinimum": 0},
    "frame_rate": {
      "type": "object",
      "additionalProperties": false,
      "required": ["numerator", "denominator"],
      "properties": {
        "numerator": {"type": "integer", "minimum": 1},
        "denominator": {"type": "integer", "minimum": 1},
        "drop_frame": {"type": "boolean", "default": false}
      }
    },
    "canvas": {
      "type": "object",
      "additionalProperties": false,
      "required": ["width", "height"],
      "properties": {
        "width": {"type": "integer", "minimum": 1, "maximum": 16384},
        "height": {"type": "integer", "minimum": 1, "maximum": 16384},
        "background": {"type": "string", "pattern": "^#[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?$"}
      }
    },
    "tracks": {
      "type": "array",
      "minItems": 1,
      "maxItems": 256,
      "items": {"$ref": "#/$defs/track"}
    },
    "markers": {
      "type": "array",
      "maxItems": 10000,
      "items": {"$ref": "#/$defs/marker"}
    },
    "metadata": {"type": "object", "maxProperties": 100}
  },
  "$defs": {
    "track": {
      "type": "object",
      "additionalProperties": false,
      "required": ["id", "kind", "name", "order", "clips"],
      "properties": {
        "id": {"type": "string", "minLength": 1, "maxLength": 128},
        "kind": {"enum": ["video", "narration", "music", "sfx", "subtitle", "overlay"]},
        "name": {"type": "string", "maxLength": 200},
        "order": {"type": "integer"},
        "muted": {"type": "boolean", "default": false},
        "visible": {"type": "boolean", "default": true},
        "locked": {"type": "boolean", "default": false},
        "language": {"type": "string", "minLength": 2, "maxLength": 35},
        "mix": {"$ref": "#/$defs/audio_mix"},
        "style": {"type": "object", "maxProperties": 100},
        "clips": {
          "type": "array",
          "maxItems": 100000,
          "items": {"$ref": "#/$defs/clip"}
        },
        "metadata": {"type": "object", "maxProperties": 100}
      }
    },
    "clip": {
      "type": "object",
      "additionalProperties": false,
      "required": ["id", "timeline_in_sec", "timeline_out_sec", "source", "origin"],
      "properties": {
        "id": {"type": "string", "minLength": 1, "maxLength": 128},
        "timeline_in_sec": {"type": "number", "minimum": 0},
        "timeline_out_sec": {"type": "number", "exclusiveMinimum": 0},
        "source": {"$ref": "#/$defs/source_ref"},
        "source_in_sec": {"type": "number", "minimum": 0},
        "source_out_sec": {"type": "number", "exclusiveMinimum": 0},
        "speed": {"type": "number", "exclusiveMinimum": 0, "maximum": 16, "default": 1},
        "loop": {"type": "boolean", "default": false},
        "freeze_frame": {"type": "boolean", "default": false},
        "transform": {"$ref": "#/$defs/transform"},
        "audio": {"$ref": "#/$defs/audio_mix"},
        "transition_in": {"$ref": "#/$defs/transition"},
        "transition_out": {"$ref": "#/$defs/transition"},
        "subtitle": {"$ref": "#/$defs/subtitle_cue"},
        "narration": {"$ref": "#/$defs/narration_ref"},
        "text": {"type": "string", "maxLength": 20000},
        "style": {"type": "object", "maxProperties": 100},
        "origin": {"enum": ["ai", "user", "imported", "system"]},
        "confidence": {"type": "number", "minimum": 0, "maximum": 1},
        "proposal_ref": {"type": "string"},
        "metadata": {"type": "object", "maxProperties": 100}
      }
    },
    "source_ref": {
      "oneOf": [
        {
          "type": "object", "additionalProperties": false,
          "required": ["type", "asset_id", "artifact_id"],
          "properties": {
            "type": {"const": "asset"},
            "asset_id": {"type": "string", "minLength": 1},
            "artifact_id": {"type": "string", "minLength": 1},
            "role": {"type": "string"}
          }
        },
        {
          "type": "object", "additionalProperties": false,
          "required": ["type", "scene_id", "asset_id", "artifact_id"],
          "properties": {
            "type": {"const": "scene"},
            "scene_id": {"type": "string", "minLength": 1},
            "asset_id": {"type": "string", "minLength": 1},
            "artifact_id": {"type": "string", "minLength": 1}
          }
        },
        {
          "type": "object", "additionalProperties": false,
          "required": ["type", "artifact_id"],
          "properties": {
            "type": {"const": "artifact"},
            "artifact_id": {"type": "string", "minLength": 1},
            "role": {"type": "string"}
          }
        },
        {
          "type": "object", "additionalProperties": false,
          "required": ["type", "generator_ref"],
          "properties": {
            "type": {"const": "generated"},
            "generator_ref": {"type": "string", "minLength": 1},
            "artifact_id": {"type": "string"}
          }
        },
        {
          "type": "object", "additionalProperties": false,
          "required": ["type", "inline_id"],
          "properties": {
            "type": {"const": "none"},
            "inline_id": {"type": "string", "minLength": 1}
          }
        }
      ]
    },
    "transform": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "position": {
          "type": "object", "additionalProperties": false,
          "required": ["x", "y", "unit", "anchor"],
          "properties": {
            "x": {"type": "number"},
            "y": {"type": "number"},
            "unit": {"enum": ["normalized", "pixels"]},
            "anchor": {"enum": ["top_left", "top", "top_right", "left", "center", "right", "bottom_left", "bottom", "bottom_right"]}
          }
        },
        "scale_x": {"type": "number", "exclusiveMinimum": 0, "maximum": 100},
        "scale_y": {"type": "number", "exclusiveMinimum": 0, "maximum": 100},
        "rotation_deg": {"type": "number", "minimum": -36000, "maximum": 36000},
        "crop": {
          "type": "object", "additionalProperties": false,
          "required": ["x", "y", "width", "height", "unit"],
          "properties": {
            "x": {"type": "number", "minimum": 0},
            "y": {"type": "number", "minimum": 0},
            "width": {"type": "number", "exclusiveMinimum": 0},
            "height": {"type": "number", "exclusiveMinimum": 0},
            "unit": {"enum": ["normalized", "pixels"]}
          }
        },
        "fit": {"enum": ["contain", "cover", "fill", "smart_reframe"]},
        "opacity": {"type": "number", "minimum": 0, "maximum": 1, "default": 1}
      }
    },
    "audio_mix": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "enabled": {"type": "boolean", "default": true},
        "volume": {"type": "number", "minimum": 0, "maximum": 4, "default": 1},
        "gain_db": {"type": "number", "minimum": -96, "maximum": 24},
        "pan": {"type": "number", "minimum": -1, "maximum": 1},
        "fade_in_sec": {"type": "number", "minimum": 0},
        "fade_out_sec": {"type": "number", "minimum": 0},
        "duck_under_track_ids": {"type": "array", "items": {"type": "string"}, "uniqueItems": true},
        "duck_db": {"type": "number", "minimum": -48, "maximum": 0}
      }
    },
    "transition": {
      "type": "object",
      "additionalProperties": false,
      "required": ["kind", "duration_sec"],
      "properties": {
        "kind": {"enum": ["cut", "fade", "crossfade", "dip_to_black", "wipe", "custom"]},
        "duration_sec": {"type": "number", "minimum": 0},
        "params": {"type": "object", "maxProperties": 50}
      }
    },
    "subtitle_cue": {
      "type": "object",
      "additionalProperties": false,
      "required": ["text", "language"],
      "properties": {
        "text": {"type": "string", "minLength": 1, "maxLength": 20000},
        "language": {"type": "string", "minLength": 2, "maxLength": 35},
        "speaker": {"type": "string", "maxLength": 200},
        "words": {"type": "array", "items": {"$ref": "#/$defs/word"}, "maxItems": 10000},
        "emphasis": {"type": "array", "items": {"type": "string"}, "maxItems": 1000}
      }
    },
    "word": {
      "type": "object",
      "additionalProperties": false,
      "required": ["text", "start_offset_sec", "end_offset_sec"],
      "properties": {
        "text": {"type": "string"},
        "start_offset_sec": {"type": "number", "minimum": 0},
        "end_offset_sec": {"type": "number", "exclusiveMinimum": 0},
        "confidence": {"type": "number", "minimum": 0, "maximum": 1}
      }
    },
    "narration_ref": {
      "type": "object",
      "additionalProperties": false,
      "required": ["narration_id", "script_version_id"],
      "properties": {
        "narration_id": {"type": "string", "minLength": 1},
        "script_version_id": {"type": "string", "minLength": 1},
        "segment_ids": {"type": "array", "items": {"type": "string"}, "uniqueItems": true}
      }
    },
    "marker": {
      "type": "object",
      "additionalProperties": false,
      "required": ["id", "time_sec", "kind"],
      "properties": {
        "id": {"type": "string", "minLength": 1},
        "time_sec": {"type": "number", "minimum": 0},
        "kind": {"enum": ["beat", "chapter", "hook", "review", "custom"]},
        "label": {"type": "string", "maxLength": 500},
        "metadata": {"type": "object", "maxProperties": 50}
      }
    }
  }
}
```

## 3. Cross-field and referential validation

JSON Schema không đủ cho các rule sau; Timeline validator bắt buộc enforce:

1. Track ID unique; Clip ID unique toàn document; `order` deterministic.
2. `0 <= timeline_in_sec < timeline_out_sec <= duration_sec` với epsilon 1 ms.
3. Với source media: `0 <= source_in_sec < source_out_sec <= exact source duration`; cả hai field phải cùng có hoặc cùng vắng.
4. Bình thường `(source_out - source_in) / speed == timeline_out - timeline_in` trong tolerance; `loop`/`freeze_frame` có rule riêng.
5. `source.type` discriminator phải resolve đúng Asset/Artifact/Scene, cùng Project và `committed|ready` theo loại.
6. Scene source Artifact phải đúng source revision của Scene; không cho Scene ID trỏ blob khác.
7. `normalized` crop yêu cầu `x,y,width,height <= 1` và `x+width <= 1`, `y+height <= 1`; pixel crop không vượt source dimensions.
8. SubtitleTrack (`kind=subtitle`) chỉ chứa Clip có `subtitle`; word offsets tăng, nằm trong clip duration và `start < end`.
9. Narration track chỉ dùng Narration/audio Artifact tương ứng exact ScriptVersion; music/BGM/SFX dùng audio source và mix policy.
10. Transition duration không âm, không vượt clip và overlap phải được renderer/profile hỗ trợ.
11. Clip overlap: video/overlay compositing theo track order; cùng track chỉ overlap khi transition/composite policy explicit. Narration overlap mặc định reject; music/SFX có thể mix.
12. Marker nằm trong duration. Track/clip/text/metadata counts và size phải nằm trong server limits.
13. `origin=user` không bị proposal merge thay nếu không có explicit command `replace_user_override=true` và actor authorization.
14. Deliverable Render yêu cầu TimelineVersion `approved|locked`; preview có thể dùng `draft|proposed` nếu request explicit.

## 4. Canonical example

```json
{
  "schema_version": "1.0",
  "timeline_id": "tl_01",
  "timeline_version_id": "tlv_03",
  "project_id": "proj_01",
  "version": 3,
  "duration_sec": 12.4,
  "frame_rate": {"numerator": 30, "denominator": 1, "drop_frame": false},
  "canvas": {"width": 1920, "height": 1080, "background": "#000000"},
  "tracks": [
    {
      "id": "track_video", "kind": "video", "name": "Footage", "order": 0,
      "clips": [{
        "id": "clip_scene_12", "timeline_in_sec": 0, "timeline_out_sec": 8.2,
        "source": {"type": "scene", "scene_id": "scene_12", "asset_id": "asset_movie", "artifact_id": "art_source_v1"},
        "source_in_sec": 248.1, "source_out_sec": 256.3, "speed": 1,
        "origin": "user", "proposal_ref": "match_44",
        "transform": {"fit": "cover", "opacity": 1, "position": {"x": 0.5, "y": 0.5, "unit": "normalized", "anchor": "center"}}
      }]
    },
    {
      "id": "track_narration", "kind": "narration", "name": "Voice-over", "order": 10,
      "clips": [{
        "id": "clip_voice_1", "timeline_in_sec": 0, "timeline_out_sec": 5.4,
        "source": {"type": "artifact", "artifact_id": "art_narration", "role": "narration_audio"},
        "source_in_sec": 0, "source_out_sec": 5.4,
        "narration": {"narration_id": "nar_01", "script_version_id": "scriptv_04", "segment_ids": ["seg_1"]},
        "audio": {"enabled": true, "volume": 1, "gain_db": 0}, "origin": "system"
      }]
    },
    {
      "id": "track_sub_vi", "kind": "subtitle", "name": "Vietnamese", "order": 20, "language": "vi",
      "clips": [{
        "id": "cue_1", "timeline_in_sec": 0, "timeline_out_sec": 2.1,
        "source": {"type": "none", "inline_id": "cue_1"},
        "subtitle": {"text": "Mọi chuyện bắt đầu từ đây.", "language": "vi"},
        "origin": "system"
      }]
    }
  ],
  "markers": [{"id": "hook", "time_sec": 0, "kind": "hook", "label": "Hook"}],
  "metadata": {"proposal_job_id": "job_01"}
}
```

## 5. Versioning and editing

- Timeline aggregate giữ current version; TimelineVersion document/content hash immutable.
- `PATCH`/replace dựa trên exact source version + `If-Match`; server validate rồi tạo version `n+1`, không mutate `n`.
- AI proposal ghi `origin=ai`, `proposal_ref`, confidence/provenance; user edit ghi `origin=user` cho region thay đổi.
- Concurrent conflict trả current version + safe diff summary; không merge array clip theo index.
- Approved/locked version edit bằng duplicate/new version. Approval/lock là audit transition, không đổi document bytes.
- Rollback nghĩa là tạo version mới dựa trên old content hoặc đổi current pointer theo guarded command; không xóa history.
- Track/Clip không có table canonical; JSONB TimelineVersion là source of truth. Projection nếu có phải rebuild được và không được renderer đọc thay document.

Patch transport còn ở OQ-08; bất kể JSON Patch/full replace/domain command, versioning rules trên không đổi.

## 6. Render profile separation

RenderProfile snapshot gồm:

```text
profile_key/version, platform, aspect_ratio, width, height, fps,
video_codec, audio_codec, pixel_format, bitrate/crf,
safe_area, subtitle_style policy, loudness target,
auto_reframe policy, preview/deliverable policy
```

Profile có thể crop/reframe theo declared transform policy nhưng không đổi Scene/Clip selection. Nếu smart reframe cần AI, kết quả reframe là profile-specific intermediate Artifact fingerprinted từ TimelineVersion + profile; không rerun research/script/matching.

## 7. Migration from V1

- V1 `matches.json` là match proposal, không phải Timeline canonical.
- Importer resolve every raw path thành Asset/Artifact trước khi tạo Clip.
- V1 `TimedSegment`/subtitle files map thành narration/subtitle Tracks.
- V1 BGM/narration/matched clips map exact source ranges khi evidence có; missing provenance tạo imported proposal + warning, không fabricate confidence.
- `metadata.json` và QA remain Artifacts.
- Renderer V1 chỉ được gọi qua adapter sau khi compiler materialize exact Timeline decision; không được tự choose match khác.

## 8. Timeline acceptance tests

- schema positive example và negative corpus;
- all cross-field/range/source/ownership rules;
- stable IDs/version/content hash and optimistic conflict;
- user override survives AI proposal rerun;
- same TimelineVersion renders 16:9, 9:16 and 1:1 without upstream AI rerun;
- changing one Clip invalidates only timeline/profile-dependent artifacts;
- renderer input trace contains exact TimelineVersion/Profile and no `matches.json` decision dependency.
