-- name: IsSchemaReady :one
SELECT EXISTS (
    SELECT 1
    FROM pg_catalog.pg_namespace
    WHERE nspname = 'bridgeyok'
)::boolean AS ready;
