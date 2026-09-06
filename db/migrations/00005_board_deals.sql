-- +goose Up
CREATE TABLE bridgeyok.board_deals (
    board_id uuid PRIMARY KEY REFERENCES bridgeyok.boards(id) ON DELETE CASCADE,
    source_record jsonb NOT NULL,
    CHECK (source_record->'provenance'->>'type' IN ('secure_random', 'deterministic', 'prepared', 'constraint')),
    CHECK (jsonb_typeof(source_record->'deal') = 'object')
);

-- +goose Down
DROP TABLE bridgeyok.board_deals;
