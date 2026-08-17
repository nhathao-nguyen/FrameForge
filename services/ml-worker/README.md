# Isolated Python ML worker boundary

The Python runtime is isolated under the `nh_media` namespace. Gate B contains only contracts,
configuration and a health-capable import shell; hosted AI, Whisper, CUDA, VLM and TTS are later.
