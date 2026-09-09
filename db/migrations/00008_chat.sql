-- +goose Up
CREATE TABLE bridgeyok.chat_messages (
 message_id uuid PRIMARY KEY,
 scope text NOT NULL CHECK (scope IN ('private', 'table')),
 conversation_id text NOT NULL,
 table_id uuid REFERENCES bridgeyok.tables(id) ON DELETE CASCADE,
 sender_user_id uuid NOT NULL REFERENCES bridgeyok.users(id) ON DELETE CASCADE,
 recipient_user_id uuid REFERENCES bridgeyok.users(id) ON DELETE CASCADE,
 content text NOT NULL CHECK (octet_length(content) BETWEEN 1 AND 16000),
 client_request_id text NOT NULL CHECK (client_request_id ~ '^[A-Za-z0-9_-]{8,64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE (sender_user_id, client_request_id),
 CHECK ((scope = 'private' AND table_id IS NULL AND recipient_user_id IS NOT NULL AND sender_user_id <> recipient_user_id)
 OR (scope = 'table' AND table_id IS NOT NULL AND recipient_user_id IS NULL))
);
CREATE INDEX chat_history_idx ON bridgeyok.chat_messages(scope, conversation_id, created_at DESC, message_id DESC);
CREATE INDEX chat_retention_idx ON bridgeyok.chat_messages(scope, created_at);
ALTER TABLE bridgeyok.chat_messages ENABLE ROW LEVEL SECURITY;
-- +goose StatementBegin
CREATE FUNCTION bridgeyok.cleanup_chat_messages() RETURNS void LANGUAGE sql SET search_path = '' AS $$
 DELETE FROM bridgeyok.chat_messages WHERE scope = 'private' AND created_at < now() - interval '90 days';
 DELETE FROM bridgeyok.chat_messages WHERE scope = 'table' AND created_at < now() - interval '14 days';
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION bridgeyok.cleanup_chat_messages() FROM PUBLIC;

-- +goose Down
DROP FUNCTION bridgeyok.cleanup_chat_messages();
DROP TABLE bridgeyok.chat_messages;
