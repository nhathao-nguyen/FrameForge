# LAN client acceptance

This is the physical second-device procedure for T550. The server-side
`tools/accept-local.ps1 -Profile lan` check is only a server-path check; it
must never be reported as the second-device result.

## Server machine

1. Determine the server's private IPv4 address and keep it fixed for the test.
2. Start the LAN profile with the exact address so the API CORS allowlist and
   web client endpoint are not silently set to localhost:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tools/dev.ps1 -Profile lan -LanServerIp <SERVER_LAN_IP> -Action restart
powershell -NoProfile -ExecutionPolicy Bypass -File tools/accept-local.ps1 -Profile lan `
  -LanBaseUrl http://<SERVER_LAN_IP>:8080 `
  -LanWebBaseUrl http://<SERVER_LAN_IP>:3000
```

The second command proves server-side LAN binding and the web path only. It
does not prove a second physical device.

## Second physical device

Copy this repository's `tools/accept-lan-client.ps1` to the other Windows
device, or run it from a clean checkout. Do not start PostgreSQL, Redis,
MinIO, Go, Python, FFmpeg or a local backend on that device. Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\accept-lan-client.ps1 `
  -ServerLanIp <SERVER_LAN_IP> `
  -EvidencePath "$env:TEMP\nh-media-lan-client.json"
```

Supported client baseline: Windows 10/11 or Windows Server 2016 and newer,
Windows PowerShell 5.1 or PowerShell 7, IPv4 connectivity to the same routed
private LAN, and outbound TCP access to the configured API/web ports. The
script has no Node, Go, Python, FFmpeg, Docker or repository dependency. It
loads the inbox `System.Net.Http` assembly explicitly for Windows PowerShell
5.1 and applies bounded REST/SSE timeouts.

The script rejects loopback/APIPA addresses and fails if the server IP is one
of the client machine's own IPv4 addresses. It records the server IP, client
machine name, client IPv4 addresses, UTC timestamps, a run ID and the
authenticated Project/Job/SSE checks. The LocalAuth password is prompted into
memory and is never written to the evidence file.

The result is a physical second-device PASS only when the evidence JSON says
`status: PASS` and `client.external_to_server: true`. Provide that JSON with
the task handoff; it contains no bearer token or password. If the device is
unavailable, leave the status as `EXTERNAL INPUT REQUIRED`/`NOT PASS`.

For the packaged Tauri client, build/sign the LAN flavor with the same exact
server address so both the frontend endpoint and CSP are pinned without a
wildcard:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tools/run-t434-signing.ps1 `
  -LanServerIp <SERVER_LAN_IP>
```

The LAN server profile allowlists both the browser origin
`http://<SERVER_LAN_IP>:3000` and the packaged Windows Tauri origin
`http://tauri.localhost`. A package built without `-LanServerIp` remains a
loopback-only package and is not LAN desktop evidence.
