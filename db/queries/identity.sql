-- name: CreateGuestSession :exec
INSERT INTO bridgeyok.guest_sessions (
    id,
    credential_hash,
    nickname,
    status,
    created_at,
    last_seen_at,
    expires_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(credential_hash),
    sqlc.arg(nickname),
    'ACTIVE',
    sqlc.arg(created_at),
    sqlc.arg(last_seen_at),
    sqlc.arg(expires_at)
);

-- name: RotateGuestCredential :one
UPDATE bridgeyok.guest_sessions
SET credential_hash = sqlc.arg(new_credential_hash),
    last_seen_at = sqlc.arg(now)
WHERE credential_hash = sqlc.arg(old_credential_hash)
  AND status = 'ACTIVE'
  AND expires_at > sqlc.arg(now)
RETURNING id, nickname, status, expires_at;

-- name: FindActiveGuestSession :one
SELECT id, nickname, status, expires_at
FROM bridgeyok.guest_sessions
WHERE id = sqlc.arg(id)
  AND status = 'ACTIVE'
  AND expires_at > sqlc.arg(now);

-- name: RevokeGuestSession :execrows
UPDATE bridgeyok.guest_sessions
SET status = 'REVOKED',
    last_seen_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND status = 'ACTIVE';

-- name: StoreRealtimeTicket :exec
INSERT INTO bridgeyok.realtime_tickets (
    ticket_hash,
    session_id,
    created_at,
    expires_at
) VALUES (
    sqlc.arg(ticket_hash),
    sqlc.arg(session_id),
    sqlc.arg(created_at),
    sqlc.arg(expires_at)
);

-- name: ConsumeRealtimeTicket :one
WITH consumed AS (
    UPDATE bridgeyok.realtime_tickets
    SET used_at = sqlc.arg(now)
    WHERE ticket_hash = sqlc.arg(ticket_hash)
      AND used_at IS NULL
      AND expires_at > sqlc.arg(now)
    RETURNING session_id
)
SELECT guest_sessions.id,
       guest_sessions.nickname,
       guest_sessions.status,
       guest_sessions.expires_at
FROM consumed
JOIN bridgeyok.guest_sessions
  ON guest_sessions.id = consumed.session_id
WHERE guest_sessions.status = 'ACTIVE'
  AND guest_sessions.expires_at > sqlc.arg(now);
