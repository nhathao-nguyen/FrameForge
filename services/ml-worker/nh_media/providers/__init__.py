"""Provider-neutral Gate G ports and deterministic adapters."""

from nh_media.gate_g import (
    FakeProvider,
    ProviderCallContext,
    ProviderDescriptor,
    ProviderError,
    ProviderKind,
    ProviderPort,
    ProviderRegistry,
    ProviderResponse,
    ProviderResultMeta,
    provider_conformance,
    resolve_with_fallback,
)

__all__ = [
    "FakeProvider",
    "ProviderCallContext",
    "ProviderDescriptor",
    "ProviderError",
    "ProviderKind",
    "ProviderPort",
    "ProviderRegistry",
    "ProviderResponse",
    "ProviderResultMeta",
    "provider_conformance",
    "resolve_with_fallback",
]
