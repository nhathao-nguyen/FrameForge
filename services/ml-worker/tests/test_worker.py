from nh_media import __version__
from nh_media.worker import health


def test_worker_namespace_and_health() -> None:
    assert __version__ == "0.1.0"
    assert health() == {"service": "ml-worker", "namespace": "nh_media", "status": "live"}
