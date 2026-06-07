package database

import (
	"context"
	"strconv"
	"strings"

	"element-skin/backend/internal/util"
)

func (db *DB) AddTextureToLibrary(ctx context.Context, userID, hash, textureType, note string, isPublic bool, model string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	created := NowMS()
	pub := 0
	if isPublic {
		pub = 1
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_textures (user_id,hash,texture_type,note,model,is_public,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
		userID, hash, textureType, note, model, pub, created); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO skin_library (skin_hash,texture_type,is_public,uploader,model,name,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
		hash, textureType, pub, userID, model, note, created); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) CountTexturesForUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_textures WHERE user_id=$1`, userID).Scan(&n)
	return n, err
}

func (db *DB) VerifyTextureOwnership(ctx context.Context, userID, hash, textureType string) (bool, error) {
	var one int
	err := db.Pool.QueryRow(ctx, `SELECT 1 FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType).Scan(&one)
	if IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

func (db *DB) GetTextureInfo(ctx context.Context, userID, hash, textureType string) (map[string]any, error) {
	var h, t, note, model string
	var created int64
	var pub int
	err := db.Pool.QueryRow(ctx, `SELECT hash,texture_type,note,model,created_at,is_public FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType).
		Scan(&h, &t, &note, &model, &created, &pub)
	if IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"hash": h, "type": t, "note": note, "model": model, "created_at": created, "is_public": pub}, nil
}

func (db *DB) ListUserTextures(ctx context.Context, userID, textureType string, limit int, lastCreated *int64, lastHash string) (map[string]any, error) {
	actual := limit + 1
	args := []any{userID}
	where := `user_id=$1`
	idx := 2
	if textureType != "" {
		where += ` AND texture_type=$2`
		args = append(args, textureType)
		idx++
	}
	if lastCreated != nil && lastHash != "" {
		where += ` AND (created_at < $` + itoa(idx) + ` OR (created_at = $` + itoa(idx) + ` AND hash < $` + itoa(idx+1) + `))`
		args = append(args, *lastCreated, lastHash)
		idx += 2
	}
	q := `SELECT hash,texture_type,note,created_at,model,is_public FROM user_textures WHERE ` + where + ` ORDER BY created_at DESC, hash DESC LIMIT $` + itoa(idx)
	args = append(args, actual)
	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := []map[string]any{}
	for rows.Next() {
		var h, t, note, model string
		var created int64
		var pub int
		if err := rows.Scan(&h, &t, &note, &created, &model, &pub); err != nil {
			return nil, err
		}
		got = append(got, map[string]any{"hash": h, "type": t, "note": note, "created_at": created, "model": model, "is_public": pub})
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	var next map[string]any
	if hasNext {
		last := got[limit-1]
		next = map[string]any{"last_created_at": last["created_at"], "last_hash": last["hash"]}
	}
	return map[string]any{"items": items, "has_next": hasNext, "next_key": next, "next_cursor": util.EncodeCursor(next), "page_size": len(items)}, rows.Err()
}

func (db *DB) ListPublicLibrary(ctx context.Context, limit int, textureType, query string, lastCreated *int64, lastHash string) (map[string]any, error) {
	actual := limit + 1
	args := []any{}
	where := `sl.is_public = 1`
	idx := 1
	if textureType != "" {
		where += ` AND sl.texture_type=$` + itoa(idx)
		args = append(args, textureType)
		idx++
	}
	if query != "" {
		where += ` AND (sl.skin_hash ILIKE $` + itoa(idx) + ` OR sl.name ILIKE $` + itoa(idx) + ` OR u.display_name ILIKE $` + itoa(idx) + `)`
		args = append(args, "%"+query+"%")
		idx++
	}
	if lastCreated != nil && lastHash != "" {
		where += ` AND (sl.created_at < $` + itoa(idx) + ` OR (sl.created_at = $` + itoa(idx) + ` AND sl.skin_hash < $` + itoa(idx+1) + `))`
		args = append(args, *lastCreated, lastHash)
		idx += 2
	}
	q := `SELECT sl.skin_hash,sl.texture_type,sl.is_public,sl.uploader,sl.created_at,sl.model,sl.name,COALESCE(u.display_name,'') FROM skin_library sl LEFT JOIN users u ON sl.uploader=u.id WHERE ` + where + ` ORDER BY sl.created_at DESC, sl.skin_hash DESC LIMIT $` + itoa(idx)
	args = append(args, actual)
	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := []map[string]any{}
	for rows.Next() {
		var h, t, uploader, model, name, display string
		var pub int
		var created int64
		if err := rows.Scan(&h, &t, &pub, &uploader, &created, &model, &name, &display); err != nil {
			return nil, err
		}
		got = append(got, map[string]any{"hash": h, "type": t, "is_public": pub == 1, "uploader": uploader, "created_at": created, "model": model, "name": name, "uploader_display_name": display, "uploader_name": display})
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	var next map[string]any
	if hasNext {
		last := got[limit-1]
		next = map[string]any{"last_created_at": last["created_at"], "last_skin_hash": last["hash"]}
	}
	return map[string]any{"items": items, "has_next": hasNext, "next_cursor": util.EncodeCursor(next), "page_size": len(items)}, rows.Err()
}

func (db *DB) AddTextureToWardrobe(ctx context.Context, userID, hash string) (bool, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var textureType, model, uploader, name string
	var pub int
	err = tx.QueryRow(ctx, `SELECT texture_type,model,uploader,name,is_public FROM skin_library WHERE skin_hash=$1`, hash).Scan(&textureType, &model, &uploader, &name, &pub)
	if IsNoRows(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if pub != 1 && uploader != userID {
		return false, nil
	}
	dstPub := 2
	if uploader == userID {
		dstPub = 1
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_textures (user_id,hash,texture_type,note,model,is_public,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, userID, hash, textureType, name, model, dstPub, NowMS()); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (db *DB) UpdateTextureNote(ctx context.Context, userID, hash, textureType, note string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE user_textures SET note=$1 WHERE user_id=$2 AND hash=$3 AND texture_type=$4`, note, userID, hash, textureType); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET name=$1 WHERE skin_hash=$2 AND uploader=$3`, note, hash, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) UpdateTextureModel(ctx context.Context, userID, hash, textureType, model string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE user_textures SET model=$1 WHERE user_id=$2 AND hash=$3 AND texture_type=$4`, model, userID, hash, textureType); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET model=$1 WHERE skin_hash=$2 AND uploader=$3`, model, hash, userID); err != nil {
		return err
	}
	if strings.EqualFold(textureType, "skin") {
		if _, err := tx.Exec(ctx, `UPDATE profiles SET texture_model=$1 WHERE skin_hash=$2 AND user_id=$3`, model, hash, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (db *DB) UpdateTexturePublic(ctx context.Context, userID, hash, textureType string, isPublic bool) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	pub := 0
	if isPublic {
		pub = 1
	}
	if _, err := tx.Exec(ctx, `UPDATE user_textures SET is_public=$1 WHERE user_id=$2 AND hash=$3 AND texture_type=$4 AND is_public != 2`, pub, userID, hash, textureType); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET is_public=$1 WHERE skin_hash=$2 AND uploader=$3`, pub, hash, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) DeleteTextureFromLibrary(ctx context.Context, userID, hash, textureType string) (bool, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var one int
	err = tx.QueryRow(ctx, `SELECT 1 FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType).Scan(&one)
	if IsNoRows(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM skin_library WHERE uploader=$1 AND skin_hash=$2`, userID, hash); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (db *DB) ListAllTextures(ctx context.Context, limit int, lastCreated *int64, lastHash, query, typeFilter string) (map[string]any, error) {
	actual := limit + 1
	args := []any{}
	where := "TRUE"
	idx := 1
	if typeFilter != "" {
		where += ` AND sl.texture_type=$` + itoa(idx)
		args = append(args, typeFilter)
		idx++
	}
	if query != "" {
		where += ` AND (sl.skin_hash ILIKE $` + itoa(idx) + ` OR sl.name ILIKE $` + itoa(idx) + ` OR u.display_name ILIKE $` + itoa(idx) + `)`
		args = append(args, "%"+query+"%")
		idx++
	}
	if lastCreated != nil && lastHash != "" {
		where += ` AND (sl.created_at < $` + itoa(idx) + ` OR (sl.created_at = $` + itoa(idx) + ` AND sl.skin_hash < $` + itoa(idx+1) + `))`
		args = append(args, *lastCreated, lastHash)
		idx += 2
	}
	q := `SELECT sl.skin_hash,sl.texture_type,sl.is_public,sl.uploader,sl.created_at,sl.model,sl.name,COALESCE(u.email,''),COALESCE(u.display_name,'') FROM skin_library sl LEFT JOIN users u ON sl.uploader=u.id WHERE ` + where + ` ORDER BY sl.created_at DESC, sl.skin_hash DESC LIMIT $` + itoa(idx)
	args = append(args, actual)
	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := []map[string]any{}
	for rows.Next() {
		var h, typ, uploader, model, name, email, display string
		var pub int
		var created int64
		if err := rows.Scan(&h, &typ, &pub, &uploader, &created, &model, &name, &email, &display); err != nil {
			return nil, err
		}
		got = append(got, map[string]any{"hash": h, "type": typ, "is_public": pub == 1, "uploader_user_id": uploader, "created_at": created, "model": model, "name": name, "uploader_email": email, "uploader_display_name": display})
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	var next map[string]any
	if hasNext {
		last := got[limit-1]
		next = map[string]any{"last_created_at": last["created_at"], "last_skin_hash": last["hash"]}
	}
	return map[string]any{"items": items, "has_next": hasNext, "next_key": next, "page_size": len(items)}, rows.Err()
}

func (db *DB) AdminUpdateTexturePublic(ctx context.Context, hash string, isPublic bool) error {
	exists, err := db.TextureExists(ctx, hash)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	pub := 0
	if isPublic {
		pub = 1
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET is_public=$1 WHERE skin_hash=$2`, pub, hash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE user_textures SET is_public=$1 WHERE hash=$2 AND is_public != 2`, pub, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) AdminUpdateTextureNote(ctx context.Context, hash, note string) error {
	exists, err := db.TextureExists(ctx, hash)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET name=$1 WHERE skin_hash=$2`, note, hash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE user_textures SET note=$1 WHERE hash=$2`, note, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) AdminUpdateTextureModel(ctx context.Context, hash, model string) error {
	exists, err := db.TextureExists(ctx, hash)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET model=$1 WHERE skin_hash=$2`, model, hash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE user_textures SET model=$1 WHERE hash=$2`, model, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) TextureExists(ctx context.Context, hash string) (bool, error) {
	var one int
	err := db.Pool.QueryRow(ctx, `SELECT 1 FROM skin_library WHERE skin_hash=$1`, hash).Scan(&one)
	if IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

func (db *DB) AdminDeleteTexture(ctx context.Context, hash, textureType, userID string, force bool) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if force {
		if _, err := tx.Exec(ctx, `DELETE FROM user_textures WHERE hash=$1 AND texture_type=$2`, hash, textureType); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM skin_library WHERE skin_hash=$1`, hash); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if userID == "" {
		return errString("per-user deletion requires user_id")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType); err != nil {
		return err
	}
	var remaining int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM user_textures WHERE hash=$1 AND texture_type=$2`, hash, textureType).Scan(&remaining); err != nil {
		return err
	}
	if remaining == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM skin_library WHERE skin_hash=$1`, hash); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
