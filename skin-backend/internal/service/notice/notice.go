package notice

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"element-skin/backend/internal/database"
	noticedb "element-skin/backend/internal/database/notice"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

const (
	TypeAnnouncement = "announcement"
	TypeSystem       = "system"

	DisplayInline = "inline"
	DisplayDetail = "detail"

	LevelInfo    = "info"
	LevelSuccess = "success"
	LevelWarning = "warning"
	LevelDanger  = "danger"

	AudienceUsers  = "users"
	AudienceAdmins = "admins"

	StatusAll       = "all"
	StatusEnabled   = "enabled"
	StatusDisabled  = "disabled"
	StatusExpired   = "expired"
	StatusScheduled = "scheduled"

	MaxTitleLen   = 80
	MaxSummaryLen = 160
	MaxContentLen = 20 * 1024
)

type Service struct {
	DB *database.DB
}

type CurrentUser struct {
	ID                   string
	CanReadAdminAudience bool
}

type ListParams struct {
	Type        string
	Status      string
	Limit       int
	Cursor      string
	IncludeRead bool
	Dashboard   bool
}

type CreateInput struct {
	Type            string `json:"type"`
	Title           string `json:"title"`
	Summary         string `json:"summary"`
	ContentMarkdown string `json:"content_markdown"`
	DisplayMode     string `json:"display_mode"`
	Level           string `json:"level"`
	LinkText        string `json:"link_text"`
	LinkURL         string `json:"link_url"`
	Audience        string `json:"audience"`
	Enabled         *bool  `json:"enabled"`
	Pinned          *bool  `json:"pinned"`
	Dismissible     *bool  `json:"dismissible"`
	StartsAt        *int64 `json:"starts_at"`
	EndsAt          *int64 `json:"ends_at"`
}

type PatchInput struct {
	Type            *string `json:"type"`
	Title           *string `json:"title"`
	Summary         *string `json:"summary"`
	ContentMarkdown *string `json:"content_markdown"`
	DisplayMode     *string `json:"display_mode"`
	Level           *string `json:"level"`
	LinkText        *string `json:"link_text"`
	LinkURL         *string `json:"link_url"`
	Audience        *string `json:"audience"`
	Enabled         *bool   `json:"enabled"`
	Pinned          *bool   `json:"pinned"`
	Dismissible     *bool   `json:"dismissible"`
	StartsAt        *int64  `json:"starts_at"`
	EndsAt          *int64  `json:"ends_at"`
	ClearStartsAt   bool    `json:"-"`
	ClearEndsAt     bool    `json:"-"`
}

type cursorState struct {
	lastPinned  *bool
	lastCreated *int64
	lastID      string
}

func (s Service) ListForUser(ctx context.Context, user CurrentUser, params ListParams) (map[string]any, error) {
	cur, err := parseCursor(params.Cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	typ := strings.TrimSpace(params.Type)
	if params.Dashboard && typ == "" {
		typ = TypeAnnouncement
	}
	if typ != "" && !validType(typ) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid type"}
	}
	return s.DB.Notices.ListForUser(ctx, noticedb.UserListOptions{
		UserID:               user.ID,
		CanReadAdminAudience: user.CanReadAdminAudience,
		Type:                 typ,
		Limit:                params.Limit,
		Now:                  database.NowMS(),
		IncludeRead:          params.IncludeRead || params.Dashboard,
		LastPinned:           cur.lastPinned,
		LastCreated:          cur.lastCreated,
		LastID:               cur.lastID,
	})
}

func (s Service) GetForUser(ctx context.Context, id string, user CurrentUser) (*model.NoticeView, error) {
	item, err := s.DB.Notices.GetForUser(ctx, id, user.ID)
	if err != nil {
		return nil, err
	}
	if item == nil || !visibleToUser(*item, user, database.NowMS()) {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	now := database.NowMS()
	if err := s.DB.Notices.MarkRead(ctx, id, user.ID, now); err != nil {
		return nil, err
	}
	if item.ReadAt == nil {
		item.ReadAt = &now
		item.Read = true
	}
	return item, nil
}

func (s Service) MarkRead(ctx context.Context, id string, user CurrentUser) error {
	item, err := s.DB.Notices.GetForUser(ctx, id, user.ID)
	if err != nil {
		return err
	}
	if item == nil || !visibleToUser(*item, user, database.NowMS()) {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	return s.DB.Notices.MarkRead(ctx, id, user.ID, database.NowMS())
}

func (s Service) Dismiss(ctx context.Context, id string, user CurrentUser) error {
	item, err := s.DB.Notices.GetForUser(ctx, id, user.ID)
	if err != nil {
		return err
	}
	if item == nil || !visibleToUser(*item, user, database.NowMS()) {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	if !item.Dismissible {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "notice is not dismissible"}
	}
	return s.DB.Notices.Dismiss(ctx, id, user.ID, database.NowMS())
}

func (s Service) ListForAdmin(ctx context.Context, params ListParams) (map[string]any, error) {
	cur, err := parseCursor(params.Cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = StatusAll
	}
	if !validStatus(status) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid status"}
	}
	typ := strings.TrimSpace(params.Type)
	if typ != "" && !validType(typ) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid type"}
	}
	return s.DB.Notices.ListForAdmin(ctx, noticedb.AdminListOptions{
		Type:        typ,
		Status:      status,
		Limit:       params.Limit,
		Now:         database.NowMS(),
		LastPinned:  cur.lastPinned,
		LastCreated: cur.lastCreated,
		LastID:      cur.lastID,
	})
}

func (s Service) Create(ctx context.Context, input CreateInput, createdBy string) (*model.Notice, error) {
	notice, err := noticeFromCreate(input, createdBy)
	if err != nil {
		return nil, err
	}
	if err := s.DB.Notices.Create(ctx, notice); err != nil {
		return nil, err
	}
	return &notice, nil
}

func (s Service) Patch(ctx context.Context, id string, input PatchInput, createdBy string) (*model.Notice, error) {
	existing, err := s.DB.Notices.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	updated := *existing
	applyPatch(&updated, input)
	newID, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	now := database.NowMS()
	updated.ID = newID
	updated.CreatedAt = now
	updated.UpdatedAt = now
	updated.CreatedBy = &createdBy
	if err := validateNotice(updated); err != nil {
		return nil, err
	}
	ok, err := s.DB.Notices.Replace(ctx, id, updated)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	return &updated, nil
}

func (s Service) Delete(ctx context.Context, id string) error {
	ok, err := s.DB.Notices.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	return nil
}

func (s Service) DeleteExpired(ctx context.Context, cutoff int64) error {
	return s.DB.Notices.DeleteExpired(ctx, cutoff)
}

func noticeFromCreate(input CreateInput, createdBy string) (model.Notice, error) {
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return model.Notice{}, err
	}
	now := database.NowMS()
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	pinned := false
	if input.Pinned != nil {
		pinned = *input.Pinned
	}
	dismissible := true
	if input.Dismissible != nil {
		dismissible = *input.Dismissible
	}
	typ := strings.TrimSpace(input.Type)
	if typ == "" {
		typ = TypeAnnouncement
	}
	displayMode := strings.TrimSpace(input.DisplayMode)
	if displayMode == "" {
		displayMode = DisplayInline
	}
	level := strings.TrimSpace(input.Level)
	if level == "" {
		level = LevelInfo
	}
	audience := strings.TrimSpace(input.Audience)
	if audience == "" {
		audience = AudienceUsers
	}
	notice := model.Notice{
		ID:              id,
		Type:            typ,
		Title:           strings.TrimSpace(input.Title),
		Summary:         strings.TrimSpace(input.Summary),
		ContentMarkdown: strings.TrimSpace(input.ContentMarkdown),
		DisplayMode:     displayMode,
		Level:           level,
		LinkText:        strings.TrimSpace(input.LinkText),
		LinkURL:         strings.TrimSpace(input.LinkURL),
		Audience:        audience,
		Enabled:         enabled,
		Pinned:          pinned,
		Dismissible:     dismissible,
		StartsAt:        input.StartsAt,
		EndsAt:          input.EndsAt,
		CreatedBy:       &createdBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return notice, validateNotice(notice)
}

func applyPatch(notice *model.Notice, input PatchInput) {
	if input.Type != nil {
		notice.Type = strings.TrimSpace(*input.Type)
	}
	if input.Title != nil {
		notice.Title = strings.TrimSpace(*input.Title)
	}
	if input.Summary != nil {
		notice.Summary = strings.TrimSpace(*input.Summary)
	}
	if input.ContentMarkdown != nil {
		notice.ContentMarkdown = strings.TrimSpace(*input.ContentMarkdown)
	}
	if input.DisplayMode != nil {
		notice.DisplayMode = strings.TrimSpace(*input.DisplayMode)
	}
	if input.Level != nil {
		notice.Level = strings.TrimSpace(*input.Level)
	}
	if input.LinkText != nil {
		notice.LinkText = strings.TrimSpace(*input.LinkText)
	}
	if input.LinkURL != nil {
		notice.LinkURL = strings.TrimSpace(*input.LinkURL)
	}
	if input.Audience != nil {
		notice.Audience = strings.TrimSpace(*input.Audience)
	}
	if input.Enabled != nil {
		notice.Enabled = *input.Enabled
	}
	if input.Pinned != nil {
		notice.Pinned = *input.Pinned
	}
	if input.Dismissible != nil {
		notice.Dismissible = *input.Dismissible
	}
	if input.StartsAt != nil {
		notice.StartsAt = input.StartsAt
	} else if input.ClearStartsAt {
		notice.StartsAt = nil
	}
	if input.EndsAt != nil {
		notice.EndsAt = input.EndsAt
	} else if input.ClearEndsAt {
		notice.EndsAt = nil
	}
}

func validateNotice(notice model.Notice) error {
	if !validType(notice.Type) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid type"}
	}
	if notice.Title == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "title is required"}
	}
	if len([]rune(notice.Title)) > MaxTitleLen {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "title too long"}
	}
	if len([]rune(notice.Summary)) > MaxSummaryLen {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "summary too long"}
	}
	if len(notice.ContentMarkdown) > MaxContentLen {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "content_markdown too long"}
	}
	if notice.DisplayMode != DisplayInline && notice.DisplayMode != DisplayDetail {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid display_mode"}
	}
	if notice.DisplayMode == DisplayDetail && notice.Summary == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "summary is required for detail notices"}
	}
	if notice.DisplayMode == DisplayDetail && notice.ContentMarkdown == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "content_markdown is required for detail notices"}
	}
	if !validLevel(notice.Level) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid level"}
	}
	if notice.Audience != AudienceUsers && notice.Audience != AudienceAdmins {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid audience"}
	}
	if (notice.LinkText == "") != (notice.LinkURL == "") {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "link_text and link_url must be provided together"}
	}
	if notice.LinkURL != "" && !safeNoticeLink(notice.LinkURL) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid link_url"}
	}
	if notice.StartsAt != nil && notice.EndsAt != nil && *notice.EndsAt <= *notice.StartsAt {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "ends_at must be greater than starts_at"}
	}
	return nil
}

func visibleToUser(item model.NoticeView, user CurrentUser, now int64) bool {
	if !item.Enabled {
		return false
	}
	if item.StartsAt != nil && *item.StartsAt > now {
		return false
	}
	if item.EndsAt != nil && *item.EndsAt <= now {
		return false
	}
	if item.Audience == AudienceAdmins && !user.CanReadAdminAudience {
		return false
	}
	return item.Audience == AudienceUsers || item.Audience == AudienceAdmins
}

func parseCursor(raw string) (cursorState, error) {
	if strings.TrimSpace(raw) == "" {
		return cursorState{}, nil
	}
	m, err := util.DecodeCursor(raw)
	if err != nil || m == nil {
		return cursorState{}, errors.New("invalid cursor")
	}
	pinned, ok := m["last_pinned"].(bool)
	if !ok {
		return cursorState{}, errors.New("invalid cursor")
	}
	created, ok := util.CursorInt64(m["last_created_at"])
	if !ok {
		return cursorState{}, errors.New("invalid cursor")
	}
	id, ok := m["last_id"].(string)
	if !ok || id == "" {
		return cursorState{}, errors.New("invalid cursor")
	}
	return cursorState{lastPinned: &pinned, lastCreated: &created, lastID: id}, nil
}

func validLevel(level string) bool {
	switch level {
	case LevelInfo, LevelSuccess, LevelWarning, LevelDanger:
		return true
	default:
		return false
	}
}

func validType(typ string) bool {
	return typ == TypeAnnouncement || typ == TypeSystem
}

func validStatus(status string) bool {
	switch status {
	case StatusAll, StatusEnabled, StatusDisabled, StatusExpired, StatusScheduled:
		return true
	default:
		return false
	}
}

func safeNoticeLink(raw string) bool {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "/") {
		return !strings.HasPrefix(raw, "//") && !strings.ContainsAny(raw, "\r\n\t")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
