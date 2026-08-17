# Reference-behavior fixtures

## T003 policy

Movie Narrator is research/reference-only. This directory may contain a dated manifest and lawful
recorded observations when they are useful for understanding behavior or setting an NH-Media
quality threshold. It is never a compatibility contract and never a runtime input.

Normal NH-Media build, test, run, deploy and release commands must work with:

- no Movie Narrator checkout;
- no `movie_narrator` Python package or import;
- no upstream CLI, REST service, worker, container or image;
- no upstream source vendored into packages or artifacts.

The current manifest is intentionally empty. No fixture is required for the pre-code gate.

## Manifest requirements

Each future fixture entry must include all of the following before it is accepted:

| Field | Requirement |
|---|---|
| `fixture_id` | Stable repository-local identifier; never a user/media secret |
| `recorded_at` | UTC date/time of observation |
| `input` | Synthetic or lawfully retained input description and checksum |
| `upstream` | URL, exact commit/tag, inspected module/behavior and recorded license identifier |
| `observation` | Output metadata/checksum or a safe locator, not a source checkout or secret URL |
| `nh_media_contract` | Independently authored behavior, tolerance and acceptance target |
| `intentional_divergence` | Explicit reason when NH-Media must differ |
| `storage_review` | Retention/license review and confirmation that source did not enter product artifacts |

Raw upstream source, executable images, credentials, presigned URLs and user data are prohibited.
If an upstream checkout is needed for research, it belongs in a disposable workspace outside this
repository and is not a test prerequisite.

## Validation contract

The JSON manifest must parse as UTF-8 JSON, use the `reference-behavior-manifest/v1` schema marker,
and keep `upstream_execution_allowed_in_normal_ci` set to `false`. Normal validation checks the
manifest and verifies that the upstream checkout/package is absent; optional comparisons are
explicitly separated from the normal test command.

For this bootstrap evidence, the following PowerShell checks passed:

```powershell
python -c "import json; p=json.load(open('tests/reference-behavior/manifest.json', encoding='utf-8')); assert p['schema_version']=='reference-behavior-manifest/v1'; assert p['upstream_execution_allowed_in_normal_ci'] is False; assert p['fixtures']==[]"
if (Test-Path -LiteralPath 'references') { throw 'upstream reference checkout must remain outside the repository' }
```

The repository-wide independence scan must treat this directory as policy/evidence only. A fixture
comparison may be added later, but it must not make CI import or execute upstream.

## Handoff

```text
Task: T003
Status: complete
Implemented boundary: reference-policy README and empty manifest only
Tests: UTF-8/JSON manifest validation and empty-upstream-environment check
Independence: upstream is optional recorded research evidence, never a build/test/runtime dependency
Security: no source checkout, secret, presigned URL or user media stored
Data/rollback: empty policy/manifest is additive and reversible
Open questions: none; future fixture retention requires provenance/license review
Next task: T004 final pre-code certification, then T100 repository skeleton
```
