package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
)

func (postgres *Postgres) CreateAccount(ctx context.Context, account identity.AccountRecord, session identity.SessionRecord) error {
	tx, err := postgres.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO bridgeyok.guest_sessions(id,credential_hash,nickname,created_at,last_seen_at,expires_at) VALUES($1,$2,$3,$4,$4,$5)`, session.ID, session.CredentialHash, session.Nickname, session.CreatedAt, session.ExpiresAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO bridgeyok.users(id,session_id,username,display_name,avatar,password_salt,password_hash) VALUES($1,$2,$3,$4,$5,$6,$7)`, account.ID, account.SessionID, account.Username, account.DisplayName, account.Avatar, account.Salt, account.PasswordHash)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return identity.ErrUsernameTaken
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (postgres *Postgres) FindAccount(ctx context.Context, username string) (identity.AccountRecord, error) {
	var account identity.AccountRecord
	err := postgres.pool.QueryRow(ctx, `SELECT id::text,session_id::text,username,display_name,avatar,password_salt,password_hash FROM bridgeyok.users WHERE username=$1`, username).Scan(&account.ID, &account.SessionID, &account.Username, &account.DisplayName, &account.Avatar, &account.Salt, &account.PasswordHash)
	return account, err
}

func (postgres *Postgres) StoreAccountSession(ctx context.Context, hash []byte, userID string, expiresAt time.Time) error {
	_, err := postgres.pool.Exec(ctx, `INSERT INTO bridgeyok.account_sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, hash, userID, expiresAt)
	return err
}

func (postgres *Postgres) AuthenticateAccount(ctx context.Context, hash []byte, now time.Time) (identity.AccountRecord, time.Time, error) {
	var account identity.AccountRecord
	var expiresAt time.Time
	err := postgres.pool.QueryRow(ctx, `SELECT u.id::text,u.session_id::text,u.username,u.display_name,u.avatar,s.expires_at,
 EXISTS(SELECT 1 FROM bridgeyok.account_sessions p WHERE p.user_id=u.id AND p.expires_at>$2 AND p.online_until>$2)
 FROM bridgeyok.account_sessions s JOIN bridgeyok.users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>$2`, hash, now).Scan(&account.ID, &account.SessionID, &account.Username, &account.DisplayName, &account.Avatar, &expiresAt, &account.Online)
	if errors.Is(err, pgx.ErrNoRows) {
		err = identity.ErrInvalidCredential
	}
	return account, expiresAt, err
}

func (postgres *Postgres) RevokeAccountSession(ctx context.Context, hash []byte) error {
	_, err := postgres.pool.Exec(ctx, `DELETE FROM bridgeyok.account_sessions WHERE token_hash=$1`, hash)
	return err
}

func (postgres *Postgres) Heartbeat(ctx context.Context, hash []byte, now time.Time) error {
	result, err := postgres.pool.Exec(ctx, `UPDATE bridgeyok.account_sessions SET online_until=$2 WHERE token_hash=$1 AND expires_at>$3`, hash, now.Add(45*time.Second), now)
	if err == nil && result.RowsAffected() != 1 {
		return identity.ErrInvalidCredential
	}
	return err
}

func (postgres *Postgres) UpdateProfile(ctx context.Context, userID, name, avatar string) (identity.Profile, error) {
	var profile identity.Profile
	tx, err := postgres.pool.Begin(ctx)
	if err != nil {
		return profile, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var sessionID string
	err = tx.QueryRow(ctx, `UPDATE bridgeyok.users SET display_name=$2,avatar=$3 WHERE id=$1 RETURNING id::text,username,display_name,avatar,session_id::text`, userID, name, avatar).Scan(&profile.ID, &profile.Username, &profile.DisplayName, &profile.Avatar, &sessionID)
	if err != nil {
		return profile, err
	}
	_, err = tx.Exec(ctx, `UPDATE bridgeyok.guest_sessions SET nickname=$2 WHERE id=$1`, sessionID, name)
	if err != nil {
		return profile, err
	}
	return profile, tx.Commit(ctx)
}

func (postgres *Postgres) SearchUsers(ctx context.Context, viewerID, query string, friendsOnly bool, now time.Time) ([]identity.Profile, error) {
	rows, err := postgres.pool.Query(ctx, `SELECT u.id::text,u.username,u.display_name,u.avatar,
 EXISTS(SELECT 1 FROM bridgeyok.account_sessions s WHERE s.user_id=u.id AND s.online_until>$4 AND s.expires_at>$4),
 EXISTS(SELECT 1 FROM bridgeyok.follows f WHERE f.follower_id=$1 AND f.followed_id=u.id),
 EXISTS(SELECT 1 FROM bridgeyok.follows f JOIN bridgeyok.follows r ON r.follower_id=f.followed_id AND r.followed_id=f.follower_id WHERE f.follower_id=$1 AND f.followed_id=u.id)
 FROM bridgeyok.users u WHERE (strpos(u.username,lower($2))>0 OR strpos(lower(u.display_name),lower($2))>0)
 AND (NOT $3 OR EXISTS(SELECT 1 FROM bridgeyok.follows f JOIN bridgeyok.follows r ON r.follower_id=f.followed_id AND r.followed_id=f.follower_id WHERE f.follower_id=$1 AND f.followed_id=u.id))
 ORDER BY u.username LIMIT 30`, viewerID, query, friendsOnly, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := []identity.Profile{}
	for rows.Next() {
		var profile identity.Profile
		if err := rows.Scan(&profile.ID, &profile.Username, &profile.DisplayName, &profile.Avatar, &profile.Online, &profile.Following, &profile.Friends); err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (postgres *Postgres) Follow(ctx context.Context, viewerID, targetID string, follow bool) error {
	if viewerID == targetID {
		return identity.ErrAccountInput
	}
	if !follow {
		_, err := postgres.pool.Exec(ctx, `DELETE FROM bridgeyok.follows WHERE follower_id=$1 AND followed_id=$2`, viewerID, targetID)
		return err
	}
	_, err := postgres.pool.Exec(ctx, `INSERT INTO bridgeyok.follows(follower_id,followed_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, viewerID, targetID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return identity.ErrAccountInput
	}
	return err
}

func (postgres *Postgres) ParticipantProfiles(ctx context.Context, viewerID, tableID string, now time.Time) ([]identity.Profile, error) {
	rows, err := postgres.pool.Query(ctx, `SELECT u.id::text,u.username,u.display_name,u.avatar,p.id::text,
 EXISTS(SELECT 1 FROM bridgeyok.account_sessions s WHERE s.user_id=u.id AND s.online_until>$3 AND s.expires_at>$3),
 EXISTS(SELECT 1 FROM bridgeyok.follows f WHERE f.follower_id=$1 AND f.followed_id=u.id),
 EXISTS(SELECT 1 FROM bridgeyok.follows f JOIN bridgeyok.follows r ON r.follower_id=f.followed_id AND r.followed_id=f.follower_id WHERE f.follower_id=$1 AND f.followed_id=u.id)
 FROM bridgeyok.table_participants p JOIN bridgeyok.users u ON u.session_id=p.session_id
 WHERE p.table_id=$2 AND p.left_at IS NULL AND EXISTS(SELECT 1 FROM bridgeyok.table_participants v JOIN bridgeyok.users vu ON vu.session_id=v.session_id WHERE v.table_id=$2 AND v.left_at IS NULL AND vu.id=$1)`, viewerID, tableID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := []identity.Profile{}
	for rows.Next() {
		var profile identity.Profile
		if err := rows.Scan(&profile.ID, &profile.Username, &profile.DisplayName, &profile.Avatar, &profile.ParticipantID, &profile.Online, &profile.Following, &profile.Friends); err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (postgres *Postgres) InvitePlayer(ctx context.Context, senderID, recipientID, tableID, code string, now time.Time) error {
	if senderID == recipientID {
		return identity.ErrAccountInput
	}
	result, err := postgres.pool.Exec(ctx, `INSERT INTO bridgeyok.player_invites(sender_id,recipient_id,table_id,invite_code,expires_at)
 SELECT $1,$2,$3,$4,$5 WHERE EXISTS(SELECT 1 FROM bridgeyok.account_sessions WHERE user_id=$2 AND online_until>$6 AND expires_at>$6)
 AND EXISTS(SELECT 1 FROM bridgeyok.table_participants p JOIN bridgeyok.users u ON u.session_id=p.session_id WHERE p.table_id=$3 AND u.id=$1 AND p.left_at IS NULL)
 AND EXISTS(SELECT 1 FROM bridgeyok.tables WHERE id=$3 AND NOT locked AND state IN ('WAITING','ACTIVE','BETWEEN_BOARDS'))
 ON CONFLICT(sender_id,recipient_id,table_id) DO UPDATE SET expires_at=EXCLUDED.expires_at,invite_code=EXCLUDED.invite_code`, senderID, recipientID, tableID, code, now.Add(10*time.Minute), now)
	if err == nil && result.RowsAffected() != 1 {
		return identity.ErrUserOffline
	}
	return err
}

func (postgres *Postgres) Invitations(ctx context.Context, recipientID string, now time.Time) ([]identity.Invitation, error) {
	rows, err := postgres.pool.Query(ctx, `SELECT u.id::text,u.username,u.display_name,u.avatar,i.table_id::text,i.invite_code,i.expires_at,
 EXISTS(SELECT 1 FROM bridgeyok.account_sessions s WHERE s.user_id=u.id AND s.online_until>$2 AND s.expires_at>$2)
 FROM bridgeyok.player_invites i JOIN bridgeyok.users u ON u.id=i.sender_id JOIN bridgeyok.tables t ON t.id=i.table_id
 WHERE i.recipient_id=$1 AND i.expires_at>$2 AND t.state IN ('WAITING','ACTIVE','BETWEEN_BOARDS') AND NOT t.locked
 ORDER BY i.expires_at DESC LIMIT 20`, recipientID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invitations := []identity.Invitation{}
	for rows.Next() {
		var invite identity.Invitation
		if err := rows.Scan(&invite.Sender.ID, &invite.Sender.Username, &invite.Sender.DisplayName, &invite.Sender.Avatar, &invite.TableID, &invite.InviteCode, &invite.ExpiresAt, &invite.Sender.Online); err != nil {
			return nil, err
		}
		invitations = append(invitations, invite)
	}
	return invitations, rows.Err()
}

func (postgres *Postgres) StoreAccountTicket(ctx context.Context, ticketHash []byte, sessionID string, accountHash []byte, createdAt, expiresAt time.Time) error {
	result, err := postgres.pool.Exec(ctx, `INSERT INTO bridgeyok.realtime_tickets(ticket_hash,session_id,account_token_hash,created_at,expires_at)
 SELECT $1,$2,$3,$4,$5 WHERE EXISTS(SELECT 1 FROM bridgeyok.account_sessions WHERE token_hash=$3 AND expires_at>$4)`, ticketHash, sessionID, accountHash, createdAt, expiresAt)
	if err == nil && result.RowsAffected() != 1 {
		return identity.ErrInvalidCredential
	}
	return err
}
