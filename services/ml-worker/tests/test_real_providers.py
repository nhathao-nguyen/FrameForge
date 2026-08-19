from __future__ import annotations

import json
from urllib.error import HTTPError

import pytest

from nh_media.providers import real
from nh_media.providers.contracts import LLMMessage, LLMRequest, ProviderCallContext, ProviderError


def _context() -> ProviderCallContext:
    return ProviderCallContext("request", "correlation", "project", "job", "step", "config", 1, timeout_sec=2)


def _request() -> LLMRequest:
    return LLMRequest("Return one word.", (LLMMessage("user", "ready", False),), max_output_tokens=8)


class _Response:
    def __init__(self, payload: bytes, status: int = 200) -> None:
        self._payload = payload
        self.status = status
        self.headers = {"x-request-id": "request-test"}

    def __enter__(self) -> "_Response":
        return self

    def __exit__(self, *_args: object) -> None:
        return None

    def read(self) -> bytes:
        return self._payload


def test_remote_policy_builds_explicit_adapters_without_secret_values(monkeypatch: pytest.MonkeyPatch) -> None:
    policy = {
        "mode": "api",
        "adapter_keys": {kind: [f"openai-{kind}"] for kind in ("llm", "vlm", "tts", "asr", "embedding")},
        "providers": {
            f"openai-{kind}": {
                "deployment": "remote",
                "endpoint": "https://api.openai.com/v1",
                "model": "test-model",
                "key_ref": "env:NH_MEDIA_TEST_API_KEY",
            }
            for kind in ("llm", "vlm", "tts", "asr", "embedding")
        },
    }
    monkeypatch.delenv("NH_MEDIA_TEST_API_KEY", raising=False)
    providers = real.build_real_providers(policy)
    assert len(providers) == 5
    assert {provider.descriptor.deployment for provider in providers} == {"remote"}
    assert all("secret" not in json.dumps(provider.descriptor.__dict__).lower() for provider in providers)


def test_missing_remote_credential_is_safe_and_non_retryable(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("NH_MEDIA_TEST_API_KEY", raising=False)
    provider = real.OpenAIChatProvider(
        "openai-llm",
        {"deployment": "remote", "endpoint": "https://api.openai.com/v1", "model": "test-model", "key_ref": "env:NH_MEDIA_TEST_API_KEY"},
    )
    with pytest.raises(ProviderError) as caught:
        provider.complete(_context(), _request())
    assert caught.value.code == "auth_failed"
    assert caught.value.retryable is False
    assert "NH_MEDIA_TEST_API_KEY" not in caught.value.safe_message


@pytest.mark.parametrize(
    ("status", "code", "retryable"),
    [(401, "auth_failed", False), (429, "rate_limited", True), (503, "unavailable", True)],
)
def test_remote_http_failures_normalize_to_port_errors(
    monkeypatch: pytest.MonkeyPatch, status: int, code: str, retryable: bool
) -> None:
    monkeypatch.setenv("NH_MEDIA_TEST_API_KEY", "test-only-value")

    def raise_http_error(*_args: object, **_kwargs: object) -> object:
        raise HTTPError("https://provider.invalid", status, "test", {}, None)

    monkeypatch.setattr(real.urlrequest, "urlopen", raise_http_error)
    provider = real.OpenAIChatProvider(
        "openai-llm",
        {"deployment": "remote", "endpoint": "https://provider.invalid/v1", "model": "test-model", "key_ref": "env:NH_MEDIA_TEST_API_KEY"},
    )
    with pytest.raises(ProviderError) as caught:
        provider.complete(_context(), _request())
    assert caught.value.code == code
    assert caught.value.retryable is retryable
    assert caught.value.provider == "openai-llm"


def test_malformed_remote_json_is_normalized(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("NH_MEDIA_TEST_API_KEY", "test-only-value")
    monkeypatch.setattr(real.urlrequest, "urlopen", lambda *_args, **_kwargs: _Response(b"not-json"))
    provider = real.OpenAIChatProvider(
        "openai-llm",
        {"deployment": "remote", "endpoint": "https://provider.invalid/v1", "model": "test-model", "key_ref": "env:NH_MEDIA_TEST_API_KEY"},
    )
    with pytest.raises(ProviderError) as caught:
        provider.complete(_context(), _request())
    assert caught.value.code == "invalid_response"
    assert caught.value.retryable is False
