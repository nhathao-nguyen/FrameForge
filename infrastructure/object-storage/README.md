# MinIO development service

MinIO is private to the Compose network, uses non-default root credentials, persists in the named
volume `nh_media_minio_data`, and bootstraps the configured private bucket. Anonymous access is
explicitly disabled. Retention is retain-until-explicit-delete for protected development objects.
