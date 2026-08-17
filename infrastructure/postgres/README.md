# PostgreSQL development service

The Compose service is private to the `nh_media_private` network and persists in the named volume
`nh_media_postgres_data`, outside the repository. Initialization creates distinct non-default app
and migration roles. The app role has connection/usage access but no schema migration privilege.

No product tables are created in Gate B; migrations belong to T200+.
