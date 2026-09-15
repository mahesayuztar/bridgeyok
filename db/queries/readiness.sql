-- name: IsSchemaReady :one
SELECT (
    to_regclass('bridgeyok.tables') IS NOT NULL
    AND to_regclass('bridgeyok.match_create_requests') IS NOT NULL
    AND to_regclass('bridgeyok.team_matches') IS NOT NULL
    AND to_regclass('bridgeyok.match_results') IS NOT NULL
    AND to_regclass('bridgeyok.users') IS NOT NULL
    AND to_regclass('bridgeyok.chat_messages') IS NOT NULL
    AND to_regprocedure('bridgeyok.cleanup_chat_messages()') IS NOT NULL
)::boolean AS ready;
