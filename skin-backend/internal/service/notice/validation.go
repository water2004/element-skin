package notice

import (
	"net/http"
	"net/url"
	"strings"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

func validateNotice(notice model.Notice) error {
	if !validType(notice.Type) {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "type", Operation: "validate", Reason: "invalid"}
	}
	if notice.Title == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_title", Operation: "validate", Reason: "required"}
	}
	if len([]rune(notice.Title)) > MaxTitleLen {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_title", Operation: "validate", Reason: "too_long"}
	}
	if len([]rune(notice.Summary)) > MaxSummaryLen {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_summary", Operation: "validate", Reason: "too_long"}
	}
	if len(notice.ContentMarkdown) > MaxContentLen {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_content", Operation: "validate", Reason: "too_long"}
	}
	if notice.DisplayMode != DisplayInline && notice.DisplayMode != DisplayDetail {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_display_mode", Operation: "validate", Reason: "invalid"}
	}
	if notice.DisplayMode == DisplayDetail && notice.Summary == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_summary", Operation: "validate", Reason: "required"}
	}
	if notice.DisplayMode == DisplayDetail && notice.ContentMarkdown == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_content", Operation: "validate", Reason: "required"}
	}
	if !validLevel(notice.Level) {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_level", Operation: "validate", Reason: "invalid"}
	}
	if notice.Audience != AudienceUsers && notice.Audience != AudienceAdmins && notice.Audience != AudienceTargeted {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_audience", Operation: "validate", Reason: "invalid"}
	}
	if (notice.LinkText == "") != (notice.LinkURL == "") {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_link", Operation: "validate", Reason: "incomplete"}
	}
	if notice.LinkURL != "" && !safeNoticeLink(notice.LinkURL) {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_link", Operation: "validate", Reason: "invalid"}
	}
	if notice.StartsAt != nil && notice.EndsAt != nil && *notice.EndsAt <= *notice.StartsAt {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "notice_time_range", Operation: "validate", Reason: "invalid"}
	}
	return nil
}

func normalizedTargetUserIDs(ids []string, audience string) ([]string, error) {
	seen := map[string]bool{}
	targets := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		targets = append(targets, id)
	}
	if audience == AudienceTargeted {
		if len(targets) == 0 {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "notice_target", Operation: "validate", Reason: "required"}
		}
		return targets, nil
	}
	if len(targets) > 0 {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "notice_target", Operation: "validate", Reason: "unsupported"}
	}
	return nil, nil
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
