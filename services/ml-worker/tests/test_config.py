import pytest

from nh_media.config import Config


def test_ml_config_isolated(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("NH_ML_MAX_CONCURRENCY", "2")
    assert Config.from_env() == Config(max_concurrency=2)


def test_ml_config_rejects_unbounded_or_invalid(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("NH_ML_MAX_CONCURRENCY", "0")
    with pytest.raises(ValueError):
        Config.from_env()
