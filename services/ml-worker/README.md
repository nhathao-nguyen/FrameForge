# Isolated Python ML worker boundary

The Python runtime is isolated under the `nh_media` namespace. The deterministic `analysis` slice
can run over Redis Streams without hosted AI, Whisper, CUDA, VLM or TTS. It consumes only the
worker-command contract and publishes a bounded worker-result envelope; PostgreSQL and Artifact
state remain outside this process.

For the local Redis hop, set `NH_MEDIA_REDIS_WORKER=1`, `NH_QUEUE_ENDPOINT`,
`NH_QUEUE_STREAM_PREFIX`, `NH_MEDIA_QUEUE_CAPABILITY=analysis` and the matching consumer-group
variables, then run `uv run --project services/ml-worker python -m nh_media.worker`.
