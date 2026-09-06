-- +goose Up
CREATE TABLE bridgeyok.board_records (
    board_id uuid PRIMARY KEY,
    table_id uuid NOT NULL,
    first_seq bigint NOT NULL CHECK (first_seq > 0),
    last_seq bigint NOT NULL CHECK (last_seq >= first_seq),
    final_revision bigint NOT NULL CHECK (final_revision > 0),
    record jsonb NOT NULL CHECK ((record->>'version' = '1') IS TRUE),
    compacted_at timestamptz NOT NULL,
    FOREIGN KEY (table_id, board_id) REFERENCES bridgeyok.boards(table_id, id) ON DELETE CASCADE
);
CREATE INDEX board_records_table_idx ON bridgeyok.board_records(table_id, last_seq);
CREATE INDEX game_events_board_start_idx ON bridgeyok.game_events(table_id, (payload->>'boardId'))
WHERE event_type = 'BOARD_STARTED';

-- +goose Down
DROP INDEX bridgeyok.game_events_board_start_idx;
DROP TABLE bridgeyok.board_records;
