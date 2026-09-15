-- name: CreateMatch :exec
INSERT INTO bridgeyok.team_matches(id, owner_session_id, status, board_count, creation_hash, created_at, updated_at)
VALUES ($1, $2, 'WAITING', $3, $4, $5, $5);

-- name: LoadMatch :one
SELECT * FROM bridgeyok.team_matches WHERE id = $1;

-- name: LockMatch :one
SELECT * FROM bridgeyok.team_matches WHERE id = $1 FOR UPDATE;

-- name: InsertMatchRoom :exec
INSERT INTO bridgeyok.match_rooms(match_id, room, table_id) VALUES ($1, $2, $3);

-- name: ListMatchRooms :many
SELECT * FROM bridgeyok.match_rooms WHERE match_id = $1 ORDER BY room;

-- name: FindTableMatch :one
SELECT * FROM bridgeyok.match_rooms WHERE table_id = $1;

-- name: InsertMatchAssignment :exec
INSERT INTO bridgeyok.match_assignments(match_id, room, seat, session_id, ready) VALUES ($1, $2, $3, $4, $5);

-- name: ListMatchAssignments :many
SELECT * FROM bridgeyok.match_assignments WHERE match_id = $1 ORDER BY room, seat;

-- name: SetMatchReady :execrows
UPDATE bridgeyok.match_assignments SET ready = $3 WHERE match_id = $1 AND session_id = $2;

-- name: InsertMatchBoard :exec
INSERT INTO bridgeyok.match_boards(match_id, id, board_number) VALUES ($1, $2, $3);

-- name: ListMatchBoards :many
SELECT * FROM bridgeyok.match_boards WHERE match_id = $1 ORDER BY board_number;

-- name: SetMatchBoardSource :execrows
UPDATE bridgeyok.match_boards SET source_record = $3 WHERE match_id = $1 AND id = $2 AND source_record IS NULL;

-- name: InsertMatchRoomBoard :exec
INSERT INTO bridgeyok.match_room_boards(match_id, board_id, room, table_id, table_board_id) VALUES ($1, $2, $3, $4, $5);

-- name: FindMatchRoomBoard :one
SELECT * FROM bridgeyok.match_room_boards WHERE table_board_id = $1;

-- name: InsertMatchResult :exec
INSERT INTO bridgeyok.match_results(match_id, board_id, room, table_board_id, score_ns, finalized_at) VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListMatchResults :many
SELECT r.* FROM bridgeyok.match_results r
JOIN bridgeyok.match_boards b ON b.match_id = r.match_id AND b.id = r.board_id
WHERE r.match_id = $1 ORDER BY r.room, b.board_number;

-- name: InsertMatchComparison :execrows
INSERT INTO bridgeyok.match_comparisons(match_id, board_id, team_a_imp, compared_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (match_id, board_id) DO NOTHING;

-- name: ListMatchComparisons :many
SELECT c.* FROM bridgeyok.match_comparisons c
JOIN bridgeyok.match_boards b ON b.match_id = c.match_id AND b.id = c.board_id
WHERE c.match_id = $1 ORDER BY b.board_number;

-- name: UpdateMatch :execrows
UPDATE bridgeyok.team_matches SET status = $2, total_imp = $3, revision = revision + 1, updated_at = $4
WHERE id = $1 AND revision = $5;

-- name: ResolveMatchPlayer :one
SELECT u.id, u.session_id, g.nickname FROM bridgeyok.users u
JOIN bridgeyok.guest_sessions g ON g.id=u.session_id
WHERE u.id=$1 AND g.status='ACTIVE' AND g.expires_at>$2;

-- name: FindMatchCreateRequest :one
SELECT * FROM bridgeyok.match_create_requests WHERE owner_session_id=$1 AND request_id=$2;

-- name: InsertMatchCreateRequest :exec
INSERT INTO bridgeyok.match_create_requests(owner_session_id,request_id,request_hash,match_id) VALUES ($1,$2,$3,$4);

-- name: ListParticipantMatchIDs :many
SELECT m.id FROM bridgeyok.team_matches m
JOIN bridgeyok.match_assignments a ON a.match_id=m.id
WHERE a.session_id=$1 ORDER BY m.updated_at DESC,m.id LIMIT 20;

-- name: CountPendingMatchInvitations :one
SELECT count(*) FROM bridgeyok.match_assignments a
JOIN bridgeyok.team_matches m ON m.id=a.match_id
WHERE a.session_id=$1 AND m.status IN ('WAITING','ACTIVE');

-- name: LockMatchPlayers :many
SELECT id,session_id FROM bridgeyok.users WHERE id=ANY(sqlc.arg(user_ids)::uuid[]) ORDER BY id FOR UPDATE;
