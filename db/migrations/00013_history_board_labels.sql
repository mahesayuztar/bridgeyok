-- +goose Up
CREATE TABLE bridgeyok.history_board_labels (
    session_id uuid NOT NULL REFERENCES bridgeyok.guest_sessions (id) ON DELETE CASCADE,
    board_id uuid NOT NULL REFERENCES bridgeyok.boards (id) ON DELETE CASCADE,
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (session_id, board_id)
);

CREATE INDEX history_board_labels_board_idx
ON bridgeyok.history_board_labels (board_id);

-- +goose Down
DROP TABLE bridgeyok.history_board_labels;
