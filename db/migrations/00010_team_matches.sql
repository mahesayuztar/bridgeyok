-- +goose Up
CREATE TABLE bridgeyok.team_matches (
    id uuid PRIMARY KEY,
    owner_session_id uuid NOT NULL REFERENCES bridgeyok.guest_sessions(id),
    status text NOT NULL CHECK (status IN ('WAITING', 'ACTIVE', 'COMPLETE')),
    board_count integer NOT NULL CHECK (board_count BETWEEN 1 AND 32),
    revision bigint NOT NULL DEFAULT 0 CHECK (revision >= 0),
    total_imp integer NOT NULL DEFAULT 0 CHECK (total_imp BETWEEN -768 AND 768),
    creation_hash bytea NOT NULL CHECK (octet_length(creation_hash) = 32),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE TABLE bridgeyok.match_rooms (
    match_id uuid NOT NULL REFERENCES bridgeyok.team_matches(id) ON DELETE CASCADE,
    room text NOT NULL CHECK (room IN ('OPEN', 'CLOSED')),
    table_id uuid NOT NULL UNIQUE REFERENCES bridgeyok.tables(id),
    PRIMARY KEY (match_id, room),
    UNIQUE (match_id, room, table_id)
);
CREATE TABLE bridgeyok.match_assignments (
    match_id uuid NOT NULL,
    room text NOT NULL,
    seat text NOT NULL CHECK (seat IN ('N', 'E', 'S', 'W')),
    session_id uuid NOT NULL REFERENCES bridgeyok.guest_sessions(id),
    ready boolean NOT NULL,
    PRIMARY KEY (match_id, room, seat),
    UNIQUE (match_id, session_id),
    FOREIGN KEY (match_id, room) REFERENCES bridgeyok.match_rooms(match_id, room) ON DELETE CASCADE
);
CREATE TABLE bridgeyok.match_boards (
    match_id uuid NOT NULL REFERENCES bridgeyok.team_matches(id) ON DELETE CASCADE,
    id uuid NOT NULL UNIQUE,
    board_number integer NOT NULL CHECK (board_number BETWEEN 1 AND 32),
    source_record jsonb,
    PRIMARY KEY (match_id, id),
    UNIQUE (match_id, board_number),
    CHECK (source_record IS NULL OR (
        jsonb_typeof(source_record->'deal') = 'object'
        AND source_record->'provenance'->>'type' IN ('secure_random', 'deterministic', 'prepared', 'constraint')
    ) IS TRUE)
);
CREATE TABLE bridgeyok.match_room_boards (
    match_id uuid NOT NULL,
    board_id uuid NOT NULL,
    room text NOT NULL,
    table_id uuid NOT NULL,
    table_board_id uuid NOT NULL UNIQUE,
    PRIMARY KEY (match_id, board_id, room),
    FOREIGN KEY (match_id, board_id) REFERENCES bridgeyok.match_boards(match_id, id) ON DELETE CASCADE,
    FOREIGN KEY (match_id, room, table_id) REFERENCES bridgeyok.match_rooms(match_id, room, table_id) ON DELETE CASCADE,
    FOREIGN KEY (table_id, table_board_id) REFERENCES bridgeyok.boards(table_id, id),
    UNIQUE (match_id, board_id, room, table_board_id)
);
CREATE TABLE bridgeyok.match_results (
    match_id uuid NOT NULL,
    board_id uuid NOT NULL,
    room text NOT NULL,
    table_board_id uuid NOT NULL UNIQUE REFERENCES bridgeyok.board_records(board_id),
    score_ns integer NOT NULL CHECK (score_ns BETWEEN -7600 AND 7600 AND score_ns % 10 = 0),
    finalized_at timestamptz NOT NULL,
    PRIMARY KEY (match_id, board_id, room),
    FOREIGN KEY (match_id, board_id, room, table_board_id)
        REFERENCES bridgeyok.match_room_boards(match_id, board_id, room, table_board_id) ON DELETE CASCADE
);
CREATE TABLE bridgeyok.match_comparisons (
    match_id uuid NOT NULL,
    board_id uuid NOT NULL,
    open_room text NOT NULL DEFAULT 'OPEN' CHECK (open_room = 'OPEN'),
    closed_room text NOT NULL DEFAULT 'CLOSED' CHECK (closed_room = 'CLOSED'),
    team_a_imp integer NOT NULL CHECK (team_a_imp BETWEEN -24 AND 24),
    compared_at timestamptz NOT NULL,
    PRIMARY KEY (match_id, board_id),
    FOREIGN KEY (match_id, board_id, open_room) REFERENCES bridgeyok.match_results(match_id, board_id, room) ON DELETE CASCADE,
    FOREIGN KEY (match_id, board_id, closed_room) REFERENCES bridgeyok.match_results(match_id, board_id, room) ON DELETE CASCADE
);
ALTER TABLE bridgeyok.team_matches ENABLE ROW LEVEL SECURITY;
ALTER TABLE bridgeyok.match_rooms ENABLE ROW LEVEL SECURITY;
ALTER TABLE bridgeyok.match_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE bridgeyok.match_boards ENABLE ROW LEVEL SECURITY;
ALTER TABLE bridgeyok.match_room_boards ENABLE ROW LEVEL SECURITY;
ALTER TABLE bridgeyok.match_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE bridgeyok.match_comparisons ENABLE ROW LEVEL SECURITY;

-- +goose StatementBegin
CREATE FUNCTION bridgeyok.guard_match_board_source() RETURNS trigger LANGUAGE plpgsql SET search_path = '' AS $$
BEGIN
    IF OLD.source_record IS NOT NULL AND NEW IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION 'match board source is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER match_board_source_immutable BEFORE UPDATE ON bridgeyok.match_boards
FOR EACH ROW EXECUTE FUNCTION bridgeyok.guard_match_board_source();

-- +goose StatementBegin
CREATE FUNCTION bridgeyok.guard_sealed_match_score() RETURNS trigger LANGUAGE plpgsql SET search_path = '' AS $$
BEGIN
    IF (NEW.id, NEW.table_id, NEW.board_number, NEW.dealer, NEW.vulnerability, NEW.ruleset_version, NEW.status, NEW.score_ns, NEW.result)
        IS DISTINCT FROM (OLD.id, OLD.table_id, OLD.board_number, OLD.dealer, OLD.vulnerability, OLD.ruleset_version, OLD.status, OLD.score_ns, OLD.result)
        AND EXISTS (SELECT 1 FROM bridgeyok.match_results WHERE table_board_id = OLD.id) THEN
        RAISE EXCEPTION 'match board result is sealed' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER sealed_match_score_immutable BEFORE UPDATE ON bridgeyok.boards
FOR EACH ROW EXECUTE FUNCTION bridgeyok.guard_sealed_match_score();
-- +goose StatementBegin
CREATE FUNCTION bridgeyok.guard_match_final_record() RETURNS trigger LANGUAGE plpgsql SET search_path = '' AS $$
BEGIN
    IF NEW IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION 'match final record is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER match_result_immutable BEFORE UPDATE ON bridgeyok.match_results
FOR EACH ROW EXECUTE FUNCTION bridgeyok.guard_match_final_record();
CREATE TRIGGER match_comparison_immutable BEFORE UPDATE ON bridgeyok.match_comparisons
FOR EACH ROW EXECUTE FUNCTION bridgeyok.guard_match_final_record();
REVOKE ALL ON FUNCTION bridgeyok.guard_match_final_record() FROM PUBLIC;
REVOKE ALL ON FUNCTION bridgeyok.guard_match_board_source() FROM PUBLIC;
REVOKE ALL ON FUNCTION bridgeyok.guard_sealed_match_score() FROM PUBLIC;

-- +goose Down
DROP TRIGGER match_result_immutable ON bridgeyok.match_results;
DROP TRIGGER match_comparison_immutable ON bridgeyok.match_comparisons;
DROP FUNCTION bridgeyok.guard_match_final_record();
DROP TRIGGER sealed_match_score_immutable ON bridgeyok.boards;
DROP FUNCTION bridgeyok.guard_sealed_match_score();
DROP TRIGGER match_board_source_immutable ON bridgeyok.match_boards;
DROP FUNCTION bridgeyok.guard_match_board_source();
DROP TABLE bridgeyok.match_comparisons;
DROP TABLE bridgeyok.match_results;
DROP TABLE bridgeyok.match_room_boards;
DROP TABLE bridgeyok.match_boards;
DROP TABLE bridgeyok.match_assignments;
DROP TABLE bridgeyok.match_rooms;
DROP TABLE bridgeyok.team_matches;
