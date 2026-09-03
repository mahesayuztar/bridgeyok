-- name: FindProcessedCommand :one
SELECT outcome
FROM bridgeyok.processed_commands
WHERE table_id = sqlc.arg(table_id)
  AND session_id = sqlc.arg(session_id)
  AND request_id = sqlc.arg(request_id);

-- name: InsertProcessedCommand :exec
INSERT INTO bridgeyok.processed_commands (
    table_id,
    session_id,
    request_id,
    command_name,
    outcome,
    revision,
    last_seq,
    processed_at,
    expires_at
) VALUES (
    sqlc.arg(table_id),
    sqlc.arg(session_id),
    sqlc.arg(request_id),
    sqlc.arg(command_name),
    sqlc.arg(outcome),
    sqlc.arg(revision),
    sqlc.arg(last_seq),
    sqlc.arg(processed_at),
    sqlc.arg(expires_at)
);

-- name: LoadGameSnapshot :one
SELECT board_id,
       schema_version,
       revision,
       last_seq,
       private_state
FROM bridgeyok.game_snapshots
WHERE table_id = sqlc.arg(table_id);

-- name: UpsertGameSnapshot :exec
INSERT INTO bridgeyok.game_snapshots (
    table_id,
    board_id,
    schema_version,
    revision,
    last_seq,
    private_state,
    updated_at
) VALUES (
    sqlc.arg(table_id),
    sqlc.narg(board_id),
    1,
    sqlc.arg(revision),
    sqlc.arg(last_seq),
    sqlc.arg(private_state),
    sqlc.arg(updated_at)
)
ON CONFLICT (table_id) DO UPDATE
SET board_id = EXCLUDED.board_id,
    schema_version = EXCLUDED.schema_version,
    revision = EXCLUDED.revision,
    last_seq = EXCLUDED.last_seq,
    private_state = EXCLUDED.private_state,
    updated_at = EXCLUDED.updated_at;

-- name: UpdateTableAfterCommand :execrows
UPDATE bridgeyok.tables
SET state = sqlc.arg(state),
    locked = sqlc.arg(locked),
    owner_session_id = sqlc.arg(owner_session_id),
    revision = sqlc.arg(next_revision),
    next_seq = sqlc.arg(next_seq),
    meaningful_at = sqlc.arg(occurred_at),
    finished_at = CASE WHEN sqlc.arg(state) = 'FINISHED' THEN sqlc.arg(occurred_at) ELSE finished_at END
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(expected_revision);

-- name: InsertGameEvent :exec
INSERT INTO bridgeyok.game_events (
    table_id,
    seq,
    revision,
    event_type,
    payload,
    occurred_at
) VALUES (
    sqlc.arg(table_id),
    sqlc.arg(seq),
    sqlc.arg(revision),
    sqlc.arg(event_type),
    sqlc.arg(payload),
    sqlc.arg(occurred_at)
);

-- name: ListGameEventsAfter :many
SELECT table_id,
       seq,
       revision,
       event_type,
       payload,
       occurred_at
FROM bridgeyok.game_events
WHERE table_id = sqlc.arg(table_id)
  AND seq > sqlc.arg(after_seq)
ORDER BY seq
LIMIT sqlc.arg(event_limit);

-- name: DeleteTableSeatsForSync :exec
DELETE FROM bridgeyok.table_seats
WHERE table_id = sqlc.arg(table_id);

-- name: ListSeatRecoveryHashes :many
SELECT participant_id,
       recovery_hash
FROM bridgeyok.table_seats
WHERE table_id = sqlc.arg(table_id);

-- name: InsertTableSeatForSync :exec
INSERT INTO bridgeyok.table_seats (
    table_id,
    seat,
    participant_id,
    ready,
    controller_epoch,
    recovery_hash,
    updated_at
) VALUES (
    sqlc.arg(table_id),
    sqlc.arg(seat),
    sqlc.arg(participant_id),
    sqlc.arg(ready),
    sqlc.arg(controller_epoch),
    sqlc.narg(recovery_hash),
    sqlc.arg(updated_at)
);

-- name: SyncParticipantLeftAt :exec
UPDATE bridgeyok.table_participants
SET left_at = sqlc.arg(left_at)
WHERE table_id = sqlc.arg(table_id)
  AND id = sqlc.arg(participant_id)
  AND left_at IS NULL;

-- name: SyncParticipantRole :exec
UPDATE bridgeyok.table_participants
SET role = sqlc.arg(role)
WHERE table_id = sqlc.arg(table_id)
  AND id = sqlc.arg(participant_id);

-- name: ExpireTableGuestSessions :exec
UPDATE bridgeyok.guest_sessions
SET status = 'EXPIRED',
    last_seen_at = sqlc.arg(expired_at)
WHERE status = 'ACTIVE'
  AND id IN (
      SELECT session_id
      FROM bridgeyok.table_participants
      WHERE table_id = sqlc.arg(table_id)
        AND left_at IS NULL
  );

-- name: UpsertBoard :exec
INSERT INTO bridgeyok.boards (
    id,
    table_id,
    board_number,
    dealer,
    vulnerability,
    ruleset_version,
    status,
    score_ns,
    result,
    created_at,
    completed_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(table_id),
    sqlc.arg(board_number),
    sqlc.arg(dealer),
    sqlc.arg(vulnerability),
    sqlc.arg(ruleset_version),
    sqlc.arg(status),
    sqlc.narg(score_ns),
    sqlc.narg(result),
    sqlc.arg(created_at),
    sqlc.narg(completed_at)
)
ON CONFLICT (table_id, id) DO UPDATE
SET status = EXCLUDED.status,
    score_ns = EXCLUDED.score_ns,
    result = EXCLUDED.result,
    completed_at = EXCLUDED.completed_at;
