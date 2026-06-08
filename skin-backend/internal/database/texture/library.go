package texture

import (
	"context"
	"errors"
	"strconv"
	"time"

	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

func (s Store) ListPublic(ctx context.Context, limit int, textureType, query string, lastCreated *int64, lastHash string) (map[string]any, error) {
	actual := limit + 1
	args := []any{}
	where := `sl.is_public = 1`
	idx := 1
	if textureType != "" {
		where += ` AND sl.texture_type=$` + strconv.Itoa(idx)
		args = append(args, textureType)
		idx++
	}
	if query != "" {
		where += ` AND (sl.skin_hash ILIKE $` + strconv.Itoa(idx) + ` OR sl.name ILIKE $` + strconv.Itoa(idx) + ` OR u.display_name ILIKE $` + strconv.Itoa(idx) + `)`
		args = append(args, "%"+query+"%")
		idx++
	}
	if lastCreated != nil && lastHash != "" {
		where += ` AND (sl.created_at < $` + strconv.Itoa(idx) + ` OR (sl.created_at = $` + strconv.Itoa(idx) + ` AND sl.skin_hash < $` + strconv.Itoa(idx+1) + `))`
		args = append(args, *lastCreated, lastHash)
		idx += 2
	}
	q := `SELECT sl.skin_hash,sl.texture_type,sl.is_public,sl.uploader,sl.created_at,sl.model,sl.name,COALESCE(u.display_name,'') FROM skin_library sl LEFT JOIN users u ON sl.uploader=u.id WHERE ` + where + ` ORDER BY sl.created_at DESC, sl.skin_hash DESC LIMIT $` + strconv.Itoa(idx)
	args = append(args, actual)
	rows, err := s.Pool.Query(ctx, q, args...)
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

func (s Store) AddToWardrobe(ctx context.Context, userID, hash string) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var textureType, model, uploader, name string
	var pub int
	err = tx.QueryRow(ctx, `SELECT texture_type,model,uploader,name,is_public FROM skin_library WHERE skin_hash=$1`, hash).Scan(&textureType, &model, &uploader, &name, &pub)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if pub != 1 {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_textures (user_id,hash,texture_type,note,model,is_public,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, userID, hash, textureType, name, model, 2, time.Now().UnixMilli()); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
