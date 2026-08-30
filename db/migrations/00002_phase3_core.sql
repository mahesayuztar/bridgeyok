-- +goose Up
CREATE TABLE bridgeyok.guest_sessions (
    id uuid PRIMARY KEY,
    credential_hash bytea NOT NULL UNIQUE CHECK (octet_length(credential_hash) = 32),
    nickname text NOT NULL CHECK (char_length(nickname) BETWEEN 2 AND 64),
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'REVOKED', 'EXPIRED')),
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL CHECK (expires_at > created_at)
);

CREATE INDEX guest_sessions_expiry_idx ON bridgeyok.guest_sessions (expires_at)
WHERE status = 'ACTIVE';

CREATE TABLE bridgeyok.tables (
    id uuid PRIMARY KEY,
    owner_session_id uuid NOT NULL REFERENCES bridgeyok.guest_sessions (id),
    invite_code_hash bytea NOT NULL UNIQUE CHECK (octet_length(invite_code_hash) = 32),
    state text NOT NULL DEFAULT 'WAITING' CHECK (state IN ('WAITING', 'ACTIVE', 'BETWEEN_BOARDS', 'FINISHED', 'EXPIRED', 'ABANDONED', 'PAUSED')),
    locked boolean NOT NULL DEFAULT false,
    revision bigint NOT NULL DEFAULT 0 CHECK (revision >= 0),
    next_seq bigint NOT NULL DEFAULT 1 CHECK (next_seq >= 1),
    created_at timestamptz NOT NULL,
    meaningful_at timestamptz NOT NULL,
    finished_at timestamptz
);

CREATE INDEX tables_lifecycle_idx ON bridgeyok.tables (state, meaningful_at);

CREATE TABLE bridgeyok.table_participants (
    id uuid PRIMARY KEY,
    table_id uuid NOT NULL REFERENCES bridgeyok.tables (id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES bridgeyok.guest_sessions (id),
    role text NOT NULL CHECK (role IN ('OWNER', 'PARTICIPANT')),
    joined_at timestamptz NOT NULL,
    left_at timestamptz,
    UNIQUE (table_id, id)
);

CREATE UNIQUE INDEX table_participants_active_session_idx
ON bridgeyok.table_participants (table_id, session_id)
WHERE left_at IS NULL;

CREATE INDEX table_participants_session_idx
ON bridgeyok.table_participants (session_id, joined_at DESC);

CREATE TABLE bridgeyok.table_seats (
    table_id uuid NOT NULL REFERENCES bridgeyok.tables (id) ON DELETE CASCADE,
    seat text NOT NULL CHECK (seat IN ('N', 'E', 'S', 'W')),
    participant_id uuid NOT NULL,
    ready boolean NOT NULL DEFAULT false,
    controller_epoch bigint NOT NULL DEFAULT 1 CHECK (controller_epoch >= 1),
    recovery_hash bytea NOT NULL CHECK (octet_length(recovery_hash) = 32),
    offline_since timestamptz,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (table_id, seat),
    UNIQUE (table_id, participant_id),
    FOREIGN KEY (table_id, participant_id)
        REFERENCES bridgeyok.table_participants (table_id, id) ON DELETE CASCADE
);

CREATE TABLE bridgeyok.boards (
    id uuid PRIMARY KEY,
    table_id uuid NOT NULL REFERENCES bridgeyok.tables (id) ON DELETE CASCADE,
    board_number integer NOT NULL CHECK (board_number >= 1),
    dealer text NOT NULL CHECK (dealer IN ('N', 'E', 'S', 'W')),
    vulnerability text NOT NULL CHECK (vulnerability IN ('NONE', 'NS', 'EW', 'BOTH')),
    ruleset_version text NOT NULL,
    status text NOT NULL CHECK (status IN ('AUCTION', 'PLAY', 'SCORED', 'PASSED_OUT')),
    score_ns integer,
    result jsonb,
    created_at timestamptz NOT NULL,
    completed_at timestamptz,
    UNIQUE (table_id, board_number),
    UNIQUE (table_id, id)
);

CREATE TABLE bridgeyok.game_snapshots (
    table_id uuid PRIMARY KEY REFERENCES bridgeyok.tables (id) ON DELETE CASCADE,
    board_id uuid,
    schema_version integer NOT NULL DEFAULT 1 CHECK (schema_version = 1),
    revision bigint NOT NULL CHECK (revision >= 0),
    last_seq bigint NOT NULL CHECK (last_seq >= 0),
    private_state jsonb NOT NULL,
    updated_at timestamptz NOT NULL,
    FOREIGN KEY (table_id, board_id)
        REFERENCES bridgeyok.boards (table_id, id)
);

CREATE TABLE bridgeyok.game_events (
    table_id uuid NOT NULL REFERENCES bridgeyok.tables (id) ON DELETE CASCADE,
    seq bigint NOT NULL CHECK (seq >= 1),
    revision bigint NOT NULL CHECK (revision >= 1),
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (table_id, seq)
);

CREATE INDEX game_events_revision_idx
ON bridgeyok.game_events (table_id, revision, seq);

CREATE TABLE bridgeyok.processed_commands (
    table_id uuid NOT NULL REFERENCES bridgeyok.tables (id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES bridgeyok.guest_sessions (id),
    request_id text NOT NULL CHECK (request_id ~ '^[A-Za-z0-9_-]{8,64}$'),
    command_name text NOT NULL,
    outcome jsonb NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 0),
    last_seq bigint NOT NULL CHECK (last_seq >= 0),
    processed_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (table_id, session_id, request_id)
);

CREATE INDEX processed_commands_expiry_idx
ON bridgeyok.processed_commands (expires_at);

CREATE TABLE bridgeyok.realtime_tickets (
    ticket_hash bytea PRIMARY KEY CHECK (octet_length(ticket_hash) = 32),
    session_id uuid NOT NULL REFERENCES bridgeyok.guest_sessions (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL CHECK (expires_at > created_at),
    used_at timestamptz
);

CREATE INDEX realtime_tickets_expiry_idx
ON bridgeyok.realtime_tickets (expires_at);

CREATE TABLE bridgeyok.abuse_reports (
    id uuid PRIMARY KEY,
    reporter_session_id uuid NOT NULL REFERENCES bridgeyok.guest_sessions (id),
    table_id uuid REFERENCES bridgeyok.tables (id),
    category text NOT NULL CHECK (category IN ('HARASSMENT', 'CHEATING', 'IMPERSONATION', 'OTHER')),
    context text NOT NULL DEFAULT '' CHECK (char_length(context) <= 500),
    status text NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'REVIEWED', 'CLOSED')),
    created_at timestamptz NOT NULL
);

CREATE TABLE bridgeyok.audit_logs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_session_id uuid REFERENCES bridgeyok.guest_sessions (id),
    action text NOT NULL,
    target_type text NOT NULL,
    target_id uuid,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE bridgeyok.audit_logs;
DROP TABLE bridgeyok.abuse_reports;
DROP TABLE bridgeyok.realtime_tickets;
DROP TABLE bridgeyok.processed_commands;
DROP TABLE bridgeyok.game_events;
DROP TABLE bridgeyok.game_snapshots;
DROP TABLE bridgeyok.boards;
DROP TABLE bridgeyok.table_seats;
DROP TABLE bridgeyok.table_participants;
DROP TABLE bridgeyok.tables;
DROP TABLE bridgeyok.guest_sessions;
