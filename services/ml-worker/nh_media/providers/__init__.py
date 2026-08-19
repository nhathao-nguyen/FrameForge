"""Typed provider-neutral Gate G ports and deterministic adapters."""

from .contracts import *  # noqa: F403
from .contracts import __all__ as _contracts_all
from .resolver import CircuitBreakerPolicy, ProviderResolver, RetryPolicy

# Real adapters are intentionally not imported here.  Selecting them is an
# explicit worker-policy action, which keeps the default conformance import
# free of heavyweight optional runtimes.

__all__ = [*_contracts_all, "CircuitBreakerPolicy", "ProviderResolver", "RetryPolicy"]
