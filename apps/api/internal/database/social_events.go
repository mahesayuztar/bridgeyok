package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
)

func (postgres *Postgres) SocialProfile(ctx context.Context, userID string) (identity.Profile, string, error) {
	var profile identity.Profile
	var sessionID string
	err := postgres.pool.QueryRow(ctx, `SELECT id::text,username,display_name,avatar,session_id::text FROM bridgeyok.users WHERE id=$1`, userID).Scan(&profile.ID, &profile.Username, &profile.DisplayName, &profile.Avatar, &sessionID)
	return profile, sessionID, err
}

func (postgres *Postgres) FollowWithEvent(ctx context.Context, viewerID, targetID string) (bool, bool, error) {
	if viewerID == targetID {
		return false, false, identity.ErrAccountInput
	}
	result, err := postgres.pool.Exec(ctx, `INSERT INTO bridgeyok.follows(follower_id,followed_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, viewerID, targetID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return false, false, identity.ErrAccountInput
		}
		return false, false, err
	}
	var mutual bool
	err = postgres.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bridgeyok.follows WHERE follower_id=$1 AND followed_id=$2)`, targetID, viewerID).Scan(&mutual)
	return result.RowsAffected() > 0, mutual, err
}
