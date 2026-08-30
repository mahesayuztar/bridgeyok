-- name: CreateTable :exec
INSERT INTO bridgeyok.tables (
    id,
    owner_session_id,
    invite_code_hash,
    state,
    locked,
    revision,
    next_seq,
    created_at,
    meaningful_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(owner_session_id),
    sqlc.arg(invite_code_hash),
    'WAITING',
    false,
    0,
    1,
    sqlc.arg(created_at),
    sqlc.arg(created_at)
);

-- name: CreateTableParticipant :exec
INSERT INTO bridgeyok.table_participants (
    id,
    table_id,
    session_id,
    role,
    joined_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(table_id),
    sqlc.arg(session_id),
    sqlc.arg(role),
    sqlc.arg(joined_at)
);

-- name: PreviewTable :one
SELECT tables.state,
       tables.locked,
       count(table_participants.id) FILTER (WHERE table_participants.left_at IS NULL)::integer AS participant_count
FROM bridgeyok.tables AS tables
LEFT JOIN bridgeyok.table_participants AS table_participants
  ON table_participants.table_id = tables.id
WHERE tables.invite_code_hash = sqlc.arg(invite_code_hash)
GROUP BY tables.id;

-- name: LockTableByInvite :one
SELECT id,
       owner_session_id,
       state,
       locked,
       revision,
       next_seq - 1 AS last_seq
FROM bridgeyok.tables
WHERE invite_code_hash = sqlc.arg(invite_code_hash)
FOR UPDATE;

-- name: LockTableByID :one
SELECT id,
       owner_session_id,
       state,
       locked,
       revision,
       next_seq - 1 AS last_seq
FROM bridgeyok.tables
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: FindTableByID :one
SELECT id,
       owner_session_id,
       state,
       locked,
       revision,
       next_seq - 1 AS last_seq
FROM bridgeyok.tables
WHERE id = sqlc.arg(id);

-- name: ListActiveTableParticipants :many
SELECT table_participants.id,
       table_participants.session_id,
       table_participants.role,
       table_participants.joined_at,
       table_participants.left_at,
       guest_sessions.nickname
FROM bridgeyok.table_participants
JOIN bridgeyok.guest_sessions
  ON guest_sessions.id = table_participants.session_id
WHERE table_participants.table_id = sqlc.arg(table_id)
  AND table_participants.left_at IS NULL
ORDER BY joined_at, table_participants.id;

-- name: ListTableSeats :many
SELECT seat,
       participant_id,
       ready,
       controller_epoch
FROM bridgeyok.table_seats
WHERE table_id = sqlc.arg(table_id)
ORDER BY seat;

-- name: MarkTableParticipantLeft :execrows
UPDATE bridgeyok.table_participants
SET left_at = sqlc.arg(left_at)
WHERE table_id = sqlc.arg(table_id)
  AND session_id = sqlc.arg(session_id)
  AND left_at IS NULL;

-- name: DeleteParticipantSeat :exec
DELETE FROM bridgeyok.table_seats
WHERE table_id = sqlc.arg(table_id)
  AND participant_id = sqlc.arg(participant_id);
