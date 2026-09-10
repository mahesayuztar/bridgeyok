-- +goose Up
DROP TABLE bridgeyok.player_invites;

-- +goose Down
CREATE TABLE bridgeyok.player_invites (
 sender_id uuid NOT NULL REFERENCES bridgeyok.users(id) ON DELETE CASCADE,
 recipient_id uuid NOT NULL REFERENCES bridgeyok.users(id) ON DELETE CASCADE,
 table_id uuid NOT NULL REFERENCES bridgeyok.tables(id) ON DELETE CASCADE,
 invite_code text NOT NULL,
 expires_at timestamptz NOT NULL,
 PRIMARY KEY(sender_id,recipient_id,table_id),
 CHECK(sender_id <> recipient_id)
);
CREATE INDEX player_invites_recipient_idx ON bridgeyok.player_invites(recipient_id,expires_at);
