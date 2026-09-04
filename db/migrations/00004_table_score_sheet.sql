-- +goose Up
CREATE TABLE bridgeyok.board_seat_attributions (
    table_id uuid NOT NULL,
    board_id uuid NOT NULL,
    seat text NOT NULL CHECK (seat IN ('N', 'E', 'S', 'W')),
    occupant_id uuid NOT NULL,
    nickname text NOT NULL CHECK (char_length(nickname) BETWEEN 2 AND 64),
    is_bot boolean NOT NULL DEFAULT false,
    PRIMARY KEY (board_id, seat),
    UNIQUE (board_id, occupant_id),
    FOREIGN KEY (table_id, board_id)
        REFERENCES bridgeyok.boards (table_id, id) ON DELETE CASCADE
);

CREATE INDEX board_seat_attributions_table_idx
ON bridgeyok.board_seat_attributions (table_id, board_id);

-- +goose Down
DROP TABLE bridgeyok.board_seat_attributions;
