-- +goose Up
CREATE TABLE bridgeyok.users (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL UNIQUE REFERENCES bridgeyok.guest_sessions(id),
    username text NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9_]{3,24}$'),
    display_name text NOT NULL,
    avatar text NOT NULL CHECK (avatar IN ('spade','heart','diamond','club','owl','fox')),
    password_salt bytea NOT NULL,
    password_hash bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE bridgeyok.account_sessions (
    token_hash bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES bridgeyok.users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    online_until timestamptz NOT NULL DEFAULT '-infinity'
);
ALTER TABLE bridgeyok.realtime_tickets ADD COLUMN account_token_hash bytea REFERENCES bridgeyok.account_sessions(token_hash) ON DELETE CASCADE;
CREATE INDEX account_sessions_user_idx ON bridgeyok.account_sessions(user_id, online_until);
CREATE TABLE bridgeyok.follows (
    follower_id uuid NOT NULL REFERENCES bridgeyok.users(id) ON DELETE CASCADE,
    followed_id uuid NOT NULL REFERENCES bridgeyok.users(id) ON DELETE CASCADE,
    PRIMARY KEY (follower_id, followed_id),
    CHECK (follower_id <> followed_id)
);
CREATE INDEX follows_reverse_idx ON bridgeyok.follows(followed_id, follower_id);
CREATE TABLE bridgeyok.player_invites (
    sender_id uuid NOT NULL REFERENCES bridgeyok.users(id) ON DELETE CASCADE,
    recipient_id uuid NOT NULL REFERENCES bridgeyok.users(id) ON DELETE CASCADE,
    table_id uuid NOT NULL REFERENCES bridgeyok.tables(id) ON DELETE CASCADE,
    invite_code text NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (sender_id, recipient_id, table_id),
    CHECK (sender_id <> recipient_id)
);
CREATE INDEX player_invites_recipient_idx ON bridgeyok.player_invites(recipient_id, expires_at);

-- +goose Down
DROP TABLE bridgeyok.player_invites;
DROP TABLE bridgeyok.follows;
ALTER TABLE bridgeyok.realtime_tickets DROP COLUMN account_token_hash;
DROP TABLE bridgeyok.account_sessions;
DROP TABLE bridgeyok.users;
