"""Generate a synthetic, rights-safe corpus narration with the real local TTS adapter."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "services" / "ml-worker"))

from nh_media.providers.contracts import ProviderCallContext, TTSRequest  # noqa: E402
from nh_media.providers.real import SapiTTSProvider  # noqa: E402


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    text = (
        "At dawn, Mira enters the quiet station and finds a map beneath the old clock. "
        "She follows the river road through the blue market, then stops when the lights go dark. "
        "By evening, a green signal appears beyond the bridge, and Mira chooses the road home."
    )
    provider = SapiTTSProvider("sapi-tts", {"deployment": "local", "voice_id": "voice-local"})
    context = ProviderCallContext("corpus", "corpus", "project_corpus", "job_corpus", "step_corpus", "config_real_local", 1, timeout_sec=120, privacy_policy="local_only")
    response = provider.synthesize(context, TTSRequest(text, "en", "voice-local", {"purpose": "rights-safe-test-corpus"}, audio_format="wav", sample_rate=16000, channels=1))
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(response.audio_blob.data)
    print(f"{output} duration_sec={response.duration_sec:.3f} bytes={len(response.audio_blob.data)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
