-- +goose Up
ALTER TABLE bridgeyok.team_matches DROP CONSTRAINT team_matches_status_check;
ALTER TABLE bridgeyok.team_matches ADD CONSTRAINT team_matches_status_check CHECK (status IN ('WAITING','ACTIVE','COMPLETE','CANCELLED'));
CREATE TABLE bridgeyok.match_create_requests (
    owner_session_id uuid NOT NULL REFERENCES bridgeyok.guest_sessions(id),
    request_id text NOT NULL CHECK (request_id ~ '^[A-Za-z0-9_-]{8,64}$'),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
    match_id uuid NOT NULL UNIQUE REFERENCES bridgeyok.team_matches(id) ON DELETE CASCADE,
    PRIMARY KEY (owner_session_id, request_id)
);
ALTER TABLE bridgeyok.match_create_requests ENABLE ROW LEVEL SECURITY;
CREATE INDEX match_assignments_session_idx ON bridgeyok.match_assignments(session_id,match_id);

-- +goose Down
DROP INDEX bridgeyok.match_assignments_session_idx;
DROP TABLE bridgeyok.match_create_requests;
ALTER TABLE bridgeyok.team_matches DROP CONSTRAINT team_matches_status_check;
ALTER TABLE bridgeyok.team_matches ADD CONSTRAINT team_matches_status_check CHECK (status IN ('WAITING','ACTIVE','COMPLETE'));
