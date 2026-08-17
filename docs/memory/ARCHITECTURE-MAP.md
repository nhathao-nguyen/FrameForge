---
last_verified: 2026-08-17
source: ../01-ARCHITECTURE.md; ../04-API-CONTRACT.md; ../13-WORKER-ARCHITECTURE.md
owner: architecture owner / task assignee
---

# Architecture map

## Runtime flow

```text
Next.js/React web ─┐
                   ├─ HTTPS Product API (Go) ─ PostgreSQL/outbox
Tauri 2 client ────┘              │              │
                                  ├─ private object storage
                                  └─ Redis delivery
                                        │
                              bounded worker controllers
                              ├─ Go media executor (FFmpeg)
                              └─ Python nh_media executor (ML/AI)
```

## Ownership

| Component | Owns | Must not own |
|---|---|---|
| Go Product API | authz, product aggregates, transactions, commands, outbox, upload orchestration | FFmpeg/ML execution, raw provider branching |
| Go media worker | media probe/transform/render orchestration | users, sessions, billing, durable product truth |
| Python `nh_media` worker | isolated ML/AI inference and analysis | Product API, auth, local-path public contracts |
| PostgreSQL | durable product and execution state | media bytes |
| Redis | bounded delivery/coordination | authoritative Job/Artifact state |
| Object storage | immutable/staged bytes | authorization decisions |
| Web/Tauri | user interaction through Product API | server secrets, direct worker/provider calls |

## Trust boundaries

1. Public/LAN clients to authenticated Product API.
2. Product API/controller to private data services and queues.
3. Controller to disposable least-privilege media/ML executor.
4. Exact-scope executor calls to object storage or an authorized provider.

## Reference boundary

Movie Narrator remains outside this graph. Research records may inform the capability matrix, but
no source checkout, image, Python namespace, API route, state mapping or adapter participates in
NH-Media runtime, build, test or deployment.
