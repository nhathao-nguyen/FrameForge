"""TTS, ASR and typed audio/script alignment contracts."""

from nh_media.gate_g import (
    Alignment,
    ArtifactRef,
    MemoryArtifactStore,
    Narration,
    ProducedBlob,
    Transcript,
    TranscriptSegment,
    TranscriptWord,
    VoiceSnapshot,
    align_audio,
    synthesize_narration,
    transcribe_segments,
    validate_timing,
)

__all__ = ["Alignment", "ArtifactRef", "MemoryArtifactStore", "Narration", "ProducedBlob", "Transcript", "TranscriptSegment", "TranscriptWord", "VoiceSnapshot", "align_audio", "synthesize_narration", "transcribe_segments", "validate_timing"]
