-- name: LoadBoardSource :one
SELECT source_record FROM bridgeyok.board_deals WHERE board_id = sqlc.arg(board_id);

-- name: ListBoardRecordEvents :many
SELECT e.seq, e.revision, e.event_type, e.payload, e.occurred_at
FROM bridgeyok.game_events e
WHERE e.table_id = sqlc.arg(table_id)
  AND e.seq >= (SELECT started.seq FROM bridgeyok.game_events started
              WHERE started.table_id = sqlc.arg(table_id) AND started.event_type = 'BOARD_STARTED'
                AND started.payload->>'boardId' = sqlc.arg(board_id)::text)
  AND e.seq <= sqlc.arg(last_seq)
ORDER BY e.seq;

-- name: InsertBoardRecord :execrows
INSERT INTO bridgeyok.board_records(board_id, table_id, first_seq, last_seq, final_revision, record, compacted_at)
VALUES (sqlc.arg(board_id), sqlc.arg(table_id), sqlc.arg(first_seq), sqlc.arg(last_seq),
        sqlc.arg(final_revision), sqlc.arg(record), sqlc.arg(compacted_at))
ON CONFLICT (board_id) DO UPDATE SET record = EXCLUDED.record
WHERE board_records.record = EXCLUDED.record
  AND board_records.table_id = EXCLUDED.table_id
  AND board_records.first_seq = EXCLUDED.first_seq
  AND board_records.last_seq = EXCLUDED.last_seq
  AND board_records.final_revision = EXCLUDED.final_revision;

-- name: LoadBoardRecord :one
SELECT * FROM bridgeyok.board_records WHERE board_id = sqlc.arg(board_id);

-- name: FindBoardRecord :one
SELECT r.record FROM bridgeyok.board_records r
WHERE r.board_id = sqlc.arg(board_id)
  AND EXISTS (SELECT 1 FROM bridgeyok.table_participants p
              WHERE p.table_id = r.table_id AND p.session_id = sqlc.arg(session_id) AND p.left_at IS NULL);

-- name: DeleteCompactedBoardEvents :exec
DELETE FROM bridgeyok.game_events e USING bridgeyok.board_records r
WHERE r.board_id = sqlc.arg(board_id) AND e.table_id = r.table_id
  AND e.seq BETWEEN r.first_seq AND r.last_seq;
