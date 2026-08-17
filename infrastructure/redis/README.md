# Redis development service

Redis is authenticated, private to the Compose network and persisted with AOF plus periodic RDB
snapshots in the named volume `nh_media_redis_data`. QueuePort semantics are intentionally deferred;
Redis is transport/coordination only and never product truth.
