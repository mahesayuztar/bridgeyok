-- +goose Up
ALTER TABLE bridgeyok.table_seats
ALTER COLUMN recovery_hash DROP NOT NULL;

-- +goose Down
ALTER TABLE bridgeyok.table_seats
ALTER COLUMN recovery_hash SET NOT NULL;
