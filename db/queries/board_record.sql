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
  AND NOT EXISTS (
      SELECT 1 FROM bridgeyok.match_rooms mr
      JOIN bridgeyok.team_matches m ON m.id = mr.match_id
      WHERE mr.table_id = r.table_id AND m.status <> 'COMPLETE'
  )
  AND EXISTS (SELECT 1 FROM bridgeyok.table_participants p
              WHERE p.table_id = r.table_id AND p.session_id = sqlc.arg(session_id));

-- name: DeleteCompactedBoardEvents :exec
DELETE FROM bridgeyok.game_events e USING bridgeyok.board_records r
WHERE r.board_id = sqlc.arg(board_id) AND e.table_id = r.table_id
  AND e.seq BETWEEN r.first_seq AND r.last_seq;

-- name: ListHistoryBoards :many
SELECT b.id,
       b.table_id,
       b.board_number,
       b.result,
       b.completed_at,
       board_label.label,
       jsonb_object_agg(
           attribution.seat,
           jsonb_build_object(
               'id', attribution.occupant_id,
               'nickname', attribution.nickname,
               'isBot', attribution.is_bot
           ) ORDER BY attribution.seat
       ) AS lineup,
       viewer_seat.seat AS viewer_seat,
       match_room_board.match_id,
       COALESCE(team_match.status, '') AS match_status,
       COALESCE(match_room_board.room, '') AS match_room,
       COALESCE(CASE
           WHEN match_assignment.room = 'OPEN' AND match_assignment.seat IN ('N', 'S') THEN 'A'
           WHEN match_assignment.room = 'CLOSED' AND match_assignment.seat IN ('E', 'W') THEN 'A'
           WHEN match_assignment.match_id IS NOT NULL THEN 'B'
           ELSE NULL
       END, '') AS team,
       comparison.team_a_imp
FROM bridgeyok.boards b
JOIN bridgeyok.board_seat_attributions attribution
  ON attribution.board_id = b.id
LEFT JOIN LATERAL (
    SELECT board_attribution.seat
    FROM bridgeyok.board_seat_attributions board_attribution
    JOIN bridgeyok.table_participants participant
      ON participant.table_id = b.table_id
     AND participant.id = board_attribution.occupant_id
     AND participant.session_id = sqlc.arg(session_id)
    WHERE board_attribution.board_id = b.id
    LIMIT 1
) viewer_seat ON true
LEFT JOIN bridgeyok.match_room_boards match_room_board
  ON match_room_board.table_board_id = b.id
LEFT JOIN bridgeyok.team_matches team_match
  ON team_match.id = match_room_board.match_id
LEFT JOIN bridgeyok.match_assignments match_assignment
  ON match_assignment.match_id = match_room_board.match_id
 AND match_assignment.session_id = sqlc.arg(session_id)
LEFT JOIN bridgeyok.match_comparisons comparison
  ON comparison.match_id = match_room_board.match_id
 AND comparison.board_id = match_room_board.board_id
LEFT JOIN bridgeyok.history_board_labels board_label
  ON board_label.board_id = b.id
 AND board_label.session_id = sqlc.arg(session_id)
WHERE b.status IN ('SCORED', 'PASSED_OUT')
  AND b.completed_at IS NOT NULL
  AND viewer_seat.seat IS NOT NULL
  AND EXISTS (
      SELECT 1
      FROM bridgeyok.table_participants participant
      WHERE participant.table_id = b.table_id
        AND participant.session_id = sqlc.arg(session_id)
  )
  AND (
      sqlc.narg(cursor_at)::timestamptz IS NULL
      OR (b.completed_at, b.id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid)
  )
  AND (
      NULLIF(trim(sqlc.arg(search)::text), '') IS NULL
      OR board_label.label ILIKE '%' || trim(sqlc.arg(search)::text) || '%'
      OR to_char(b.completed_at AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM-DD') ILIKE '%' || trim(sqlc.arg(search)::text) || '%'
      OR to_char(b.completed_at AT TIME ZONE 'Asia/Jakarta', 'DD/MM/YYYY') ILIKE '%' || trim(sqlc.arg(search)::text) || '%'
      OR to_char(b.completed_at AT TIME ZONE 'Asia/Jakarta', 'DD-MM-YYYY') ILIKE '%' || trim(sqlc.arg(search)::text) || '%'
      OR b.result::text ILIKE '%' || trim(sqlc.arg(search)::text) || '%'
      OR lower(regexp_replace(
          CASE
              WHEN b.result->>'passedOut' = 'true' THEN 'passedout'
              ELSE concat(
                  b.result->'contract'->>'level',
                  CASE b.result->'contract'->>'strain'
                      WHEN 'NT' THEN 'NT'
                      ELSE b.result->'contract'->>'strain'
                  END,
                  CASE b.result->'contract'->>'doubling'
                      WHEN 'DOUBLED' THEN 'X'
                      WHEN 'REDOUBLED' THEN 'XX'
                      ELSE ''
                  END,
                  b.result->'contract'->>'declarer',
                  CASE
                      WHEN (b.result->>'tricksDeclarer')::integer - (6 + (b.result->'contract'->>'level')::integer) = 0 THEN '='
                      WHEN (b.result->>'tricksDeclarer')::integer - (6 + (b.result->'contract'->>'level')::integer) > 0 THEN '+' || ((b.result->>'tricksDeclarer')::integer - (6 + (b.result->'contract'->>'level')::integer))::text
                      ELSE ((b.result->>'tricksDeclarer')::integer - (6 + (b.result->'contract'->>'level')::integer))::text
                  END
              )
          END,
          '\s+', '', 'g'
      )) LIKE '%' || lower(regexp_replace(
          replace(replace(replace(replace(trim(sqlc.arg(search)::text), '♣', 'C'), '♦', 'D'), '♥', 'H'), '♠', 'S'),
          '\s+', '', 'g'
      )) || '%'
  )
GROUP BY b.id, b.table_id, b.board_number, b.result, b.completed_at,
         board_label.label, viewer_seat.seat, match_room_board.match_id, team_match.status,
         match_room_board.room, match_assignment.room, match_assignment.seat,
         match_assignment.match_id, comparison.team_a_imp
ORDER BY b.completed_at DESC, b.id DESC
LIMIT sqlc.arg(page_limit);

-- name: UpsertHistoryBoardLabel :execrows
INSERT INTO bridgeyok.history_board_labels (session_id, board_id, label, updated_at)
SELECT sqlc.arg(session_id), sqlc.arg(board_id), sqlc.arg(label), sqlc.arg(updated_at)
WHERE EXISTS (
    SELECT 1
    FROM bridgeyok.board_seat_attributions attribution
    JOIN bridgeyok.table_participants participant
      ON participant.table_id = attribution.table_id
     AND participant.id = attribution.occupant_id
     AND participant.session_id = sqlc.arg(session_id)
    WHERE attribution.board_id = sqlc.arg(board_id)
)
ON CONFLICT (session_id, board_id) DO UPDATE SET
    label = EXCLUDED.label,
    updated_at = EXCLUDED.updated_at;

-- name: DeleteHistoryBoardLabel :execrows
DELETE FROM bridgeyok.history_board_labels label
WHERE label.session_id = sqlc.arg(session_id)
  AND label.board_id = sqlc.arg(board_id)
  AND EXISTS (
      SELECT 1
      FROM bridgeyok.board_seat_attributions attribution
      JOIN bridgeyok.table_participants participant
        ON participant.table_id = attribution.table_id
       AND participant.id = attribution.occupant_id
       AND participant.session_id = sqlc.arg(session_id)
      WHERE attribution.board_id = sqlc.arg(board_id)
  );
