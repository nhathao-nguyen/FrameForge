# NH-Media

Independent local/LAN-first AI video production foundation.

The repository is split into a Go Product API, Go media worker, isolated Python `nh_media` worker,
shared contracts, SDK and thin clients. Movie Narrator is research/reference-only and is not a
build, runtime, test or deployment dependency.

## PowerShell entry point

Use `tools\dev.ps1` for the canonical developer workflow. The `local` profile is loopback-only;
LAN exposure requires an explicit `lan` profile. Set required non-default credentials in the process
environment before starting infrastructure; `.env.example` contains placeholders only.

```powershell
.\tools\dev.ps1 -Profile local -Action infra-up
.\tools\dev.ps1 -Profile local -Action status
```

Gate B contains no product/domain workflow implementation.
