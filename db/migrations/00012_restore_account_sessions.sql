-- +goose Up
UPDATE bridgeyok.guest_sessions
SET status = 'ACTIVE'
WHERE status = 'EXPIRED'
  AND expires_at > now()
  AND EXISTS (
      SELECT 1
      FROM bridgeyok.users
      WHERE users.session_id = guest_sessions.id
  );

-- +goose Down
SELECT 1;
