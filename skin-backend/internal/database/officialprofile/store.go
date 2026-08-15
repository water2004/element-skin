package officialprofile

import (
	"context"
	"errors"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

type View struct {
	Binding         model.OfficialProfileBinding
	Profile         model.Profile
	IdentityLabel   string
	ProviderID      string
	ProviderName    string
	ProviderAdapter string
}

type SyncInput struct {
	ID              string
	UserID          string
	RemoteName      string
	RemoteSkinURL   string
	RemoteCapeURL   string
	RemoteSkinModel string
	SkinHash        *string
	CapeHash        *string
	SyncedAt        int64
}

type CreateInput struct {
	Binding      model.OfficialProfileBinding
	UserID       string
	ProfileNames []string
	ProfileModel string
}

var (
	ErrProfileOwnedByAnotherUser = errors.New("profile is owned by another user")
	ErrProfileNameUnavailable    = errors.New("profile name is unavailable")
)

const viewColumns = `
	b.id,b.identity_id,b.profile_id,b.remote_uuid,b.remote_name,b.remote_skin_url,b.remote_cape_url,
	b.remote_skin_model,b.created_at,b.updated_at,b.last_synced_at,
	p.id,p.user_id,p.name,p.texture_model,p.skin_hash,p.cape_hash,
	ei.label,ip.id,ip.name,ip.adapter`

type rowScanner interface {
	Scan(...any) error
}

func scanView(row rowScanner) (*View, error) {
	var item View
	err := row.Scan(
		&item.Binding.ID, &item.Binding.IdentityID, &item.Binding.ProfileID,
		&item.Binding.RemoteUUID, &item.Binding.RemoteName, &item.Binding.RemoteSkinURL,
		&item.Binding.RemoteCapeURL, &item.Binding.RemoteSkinModel, &item.Binding.CreatedAt,
		&item.Binding.UpdatedAt, &item.Binding.LastSyncedAt,
		&item.Profile.ID, &item.Profile.UserID, &item.Profile.Name, &item.Profile.TextureModel,
		&item.Profile.SkinHash, &item.Profile.CapeHash,
		&item.IdentityLabel, &item.ProviderID, &item.ProviderName, &item.ProviderAdapter,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s Store) Create(ctx context.Context, input CreateInput) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	item := input.Binding
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, item.RemoteUUID); err != nil {
		return err
	}

	var ownerID string
	err = tx.QueryRow(ctx, `SELECT user_id FROM profiles WHERE id=$1`, item.RemoteUUID).Scan(&ownerID)
	switch {
	case err == nil && ownerID != input.UserID:
		return ErrProfileOwnedByAnotherUser
	case err == nil:
		item.ProfileID = item.RemoteUUID
	case !errors.Is(err, pgx.ErrNoRows):
		return err
	default:
		created := false
		for _, name := range input.ProfileNames {
			var profileID string
			err = tx.QueryRow(ctx, `
				INSERT INTO profiles (id,user_id,name,texture_model)
				VALUES ($1,$2,$3,$4)
				ON CONFLICT DO NOTHING
				RETURNING id
			`, item.RemoteUUID, input.UserID, name, input.ProfileModel).Scan(&profileID)
			if err == nil {
				created = true
				item.ProfileID = profileID
				break
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err := tx.QueryRow(ctx, `SELECT user_id FROM profiles WHERE id=$1`, item.RemoteUUID).Scan(&ownerID); err == nil {
				if ownerID != input.UserID {
					return ErrProfileOwnedByAnotherUser
				}
				item.ProfileID = item.RemoteUUID
				created = true
				break
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if !created {
			return ErrProfileNameUnavailable
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO official_profile_bindings
			(id,identity_id,profile_id,remote_uuid,remote_name,remote_skin_url,remote_cape_url,
			 remote_skin_model,created_at,updated_at,last_synced_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, item.ID, item.IdentityID, item.ProfileID, item.RemoteUUID, item.RemoteName,
		item.RemoteSkinURL, item.RemoteCapeURL, item.RemoteSkinModel, item.CreatedAt,
		item.UpdatedAt, item.LastSyncedAt)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) GetByIDAndUser(ctx context.Context, id, userID string) (*View, error) {
	item, err := scanView(s.Pool.QueryRow(ctx, `
		SELECT `+viewColumns+`
		FROM official_profile_bindings b
		JOIN external_identities ei ON ei.id=b.identity_id
		JOIN identity_providers ip ON ip.id=ei.provider_id
		JOIN profiles p ON p.id=b.profile_id
		WHERE b.id=$1 AND ei.user_id=$2
	`, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (s Store) ListByUser(ctx context.Context, userID string) ([]View, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+viewColumns+`
		FROM official_profile_bindings b
		JOIN external_identities ei ON ei.id=b.identity_id
		JOIN identity_providers ip ON ip.id=ei.provider_id
		JOIN profiles p ON p.id=b.profile_id
		WHERE ei.user_id=$1
		ORDER BY b.created_at DESC,b.id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]View, 0)
	for rows.Next() {
		item, err := scanView(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s Store) DeleteByIDAndUser(ctx context.Context, id, userID string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM official_profile_bindings b
		USING external_identities ei
		WHERE b.id=$1 AND b.identity_id=ei.id AND ei.user_id=$2
	`, id, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s Store) Sync(ctx context.Context, input SyncInput) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var profileID string
	err = tx.QueryRow(ctx, `
		SELECT b.profile_id
		FROM official_profile_bindings b
		JOIN external_identities ei ON ei.id=b.identity_id
		WHERE b.id=$1 AND ei.user_id=$2
		FOR UPDATE OF b
	`, input.ID, input.UserID).Scan(&profileID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := addTexture(ctx, tx, input.UserID, input.SkinHash, "skin", input.RemoteSkinModel, input.SyncedAt); err != nil {
		return false, err
	}
	if err := addTexture(ctx, tx, input.UserID, input.CapeHash, "cape", "default", input.SyncedAt); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE profiles
		SET name=$1,texture_model=$2,skin_hash=$3,cape_hash=$4
		WHERE id=$5
	`, input.RemoteName, input.RemoteSkinModel, input.SkinHash, input.CapeHash, profileID); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE official_profile_bindings
		SET remote_name=$1,remote_skin_url=$2,remote_cape_url=$3,remote_skin_model=$4,
			updated_at=$5,last_synced_at=$5
		WHERE id=$6
	`, input.RemoteName, input.RemoteSkinURL, input.RemoteCapeURL, input.RemoteSkinModel,
		input.SyncedAt, input.ID); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func addTexture(ctx context.Context, tx pgx.Tx, userID string, hash *string, textureType, textureModel string, createdAt int64) error {
	if hash == nil || *hash == "" {
		return nil
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO user_textures (user_id,hash,texture_type,note,model,is_public,created_at)
		VALUES ($1,$2,$3,'',$4,0,$5)
		ON CONFLICT DO NOTHING
	`, userID, *hash, textureType, textureModel, createdAt)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO skin_library
			(skin_hash,texture_type,is_public,uploader,model,name,created_at,usage_count)
		VALUES ($1,$2,0,$3,$4,'',$5,1)
		ON CONFLICT DO NOTHING
	`, *hash, textureType, userID, textureModel, createdAt); err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		_, err = tx.Exec(ctx, `
			UPDATE skin_library
			SET usage_count=(SELECT COUNT(*) FROM user_textures WHERE hash=$1 AND texture_type=$2)
			WHERE skin_hash=$1 AND texture_type=$2
		`, *hash, textureType)
	}
	return err
}
