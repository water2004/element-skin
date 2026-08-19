package oauth

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	dboauth "element-skin/backend/internal/database/oauth"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/util"
)

const (
	reviewNoticeTTL      = 30 * 24 * time.Hour
	maxReviewReasonRunes = 500
)

func (s Service) notifyAdminsClientSubmitted(ctx context.Context, client model.OAuthClient) error {
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           fitNoticeTitle("第三方应用待审核：", client.Name),
		Summary:         fitNoticeText(fmt.Sprintf("开发者提交了第三方应用 %s，请前往管理面板审核。", client.Name), noticesvc.MaxSummaryLen),
		ContentMarkdown: fmt.Sprintf("第三方应用 `%s` 已提交审核。\n\n应用 ID：`%s`", client.Name, client.ID),
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelWarning,
		LinkText:        "前往审核",
		LinkURL:         "/admin/oauth-apps",
		Audience:        noticesvc.AudienceAdmins,
		EndsAt:          noticeExpiresAt(),
	})
	return err
}

func (s Service) notifyOwnerReviewResult(ctx context.Context, client model.OAuthClient, status, reason string) error {
	titlePrefix := "第三方应用状态更新："
	level := noticesvc.LevelInfo
	summary := fmt.Sprintf("你的第三方应用 %s 状态已更新。", client.Name)
	content := fmt.Sprintf("你的第三方应用 `%s` 状态已更新为：%s。", client.Name, reviewStatusLabel(status))
	switch status {
	case StatusActive:
		titlePrefix = "第三方应用审核通过："
		level = noticesvc.LevelSuccess
		summary = fmt.Sprintf("你的第三方应用 %s 已通过审核。", client.Name)
		content = fmt.Sprintf("你的第三方应用 `%s` 已通过审核，可以开始使用 OAuth 授权能力。", client.Name)
	case StatusRejected:
		titlePrefix = "第三方应用审核驳回："
		level = noticesvc.LevelDanger
		summary = fmt.Sprintf("你的第三方应用 %s 未通过审核。", client.Name)
		content = fmt.Sprintf("你的第三方应用 `%s` 未通过审核。\n\n原因：\n\n%s", client.Name, reason)
	case StatusDisabled:
		titlePrefix = "第三方应用已停用："
		level = noticesvc.LevelWarning
		summary = fmt.Sprintf("你的第三方应用 %s 已被管理员停用。", client.Name)
		content = fmt.Sprintf("你的第三方应用 `%s` 已被管理员停用。\n\n原因：\n\n%s", client.Name, reason)
	}
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           fitNoticeTitle(titlePrefix, client.Name),
		Summary:         fitNoticeText(summary, noticesvc.MaxSummaryLen),
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           level,
		LinkText:        "查看应用",
		LinkURL:         "/dashboard/oauth",
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          noticeExpiresAt(),
		TargetUserIDs:   []string{client.OwnerUserID},
	})
	return err
}

func (s Service) notifyPermissionDependencyChanges(ctx context.Context, result PermissionDependencyResult) error {
	grantGroups := map[string][]dboauth.RevokedGrantDependency{}
	for _, grant := range result.RevokedGrants {
		grantGroups[grant.UserID] = append(grantGroups[grant.UserID], grant)
	}
	for _, userID := range sortedKeys(grantGroups) {
		if err := s.notifyRevokedGrantDependencies(ctx, userID, grantGroups[userID]); err != nil {
			return err
		}
	}

	clientGroups := map[string][]dboauth.DisabledClientDependency{}
	for _, client := range result.DisabledClients {
		clientGroups[client.OwnerUserID] = append(clientGroups[client.OwnerUserID], client)
	}
	for _, userID := range sortedKeys(clientGroups) {
		if err := s.notifyDisabledClientDependencies(ctx, userID, clientGroups[userID]); err != nil {
			return err
		}
	}
	return nil
}

func (s Service) notifyRevokedGrantDependencies(ctx context.Context, userID string, grants []dboauth.RevokedGrantDependency) error {
	if len(grants) == 0 {
		return nil
	}
	content := "你的站点权限发生变化，以下第三方应用授权已自动撤销：\n\n" +
		revokedGrantListMarkdown(grants) +
		"\n这些授权包含你当前已不再拥有的权限，后续访问会失败。需要继续使用时，请在权限恢复后重新授权。"
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "第三方应用授权已自动撤销",
		Summary:         fitNoticeText(fmt.Sprintf("你的权限发生变化，%d 个第三方应用授权已自动撤销。", len(grants)), noticesvc.MaxSummaryLen),
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelWarning,
		LinkText:        "查看授权",
		LinkURL:         "/dashboard/oauth",
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          noticeExpiresAt(),
		TargetUserIDs:   []string{userID},
	})
	return err
}

func (s Service) notifyDisabledClientDependencies(ctx context.Context, userID string, clients []dboauth.DisabledClientDependency) error {
	if len(clients) == 0 {
		return nil
	}
	content := "你的站点权限发生变化，以下第三方应用已自动停用：\n\n" +
		disabledClientListMarkdown(clients) +
		"\n这些应用申请了你当前已不再拥有的权限。请调整应用权限后重新提交审核。"
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "第三方应用已自动停用",
		Summary:         fitNoticeText(fmt.Sprintf("你的权限发生变化，%d 个你创建的第三方应用已自动停用。", len(clients)), noticesvc.MaxSummaryLen),
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelWarning,
		LinkText:        "查看应用",
		LinkURL:         "/dashboard/oauth",
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          noticeExpiresAt(),
		TargetUserIDs:   []string{userID},
	})
	return err
}

func validateReviewReason(status, reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if status == StatusRejected || status == StatusDisabled {
		if reason == "" {
			return "", util.HTTPError{Status: http.StatusBadRequest, Object: "audit_reason", Operation: "validate", Reason: "required"}
		}
	}
	if len([]rune(reason)) > maxReviewReasonRunes {
		return "", util.HTTPError{Status: http.StatusBadRequest, Object: "audit_reason", Operation: "validate", Reason: "too_long"}
	}
	return reason, nil
}

func noticeExpiresAt() *int64 {
	expiresAt := database.NowMS() + int64(reviewNoticeTTL/time.Millisecond)
	return &expiresAt
}

func reviewStatusLabel(status string) string {
	switch status {
	case StatusActive:
		return "已通过"
	case StatusRejected:
		return "已驳回"
	case StatusDisabled:
		return "已停用"
	default:
		return status
	}
}

func fitNoticeTitle(prefix, value string) string {
	return fitNoticeText(prefix+value, noticesvc.MaxTitleLen)
}

func fitNoticeText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-1]) + "…"
}

func revokedGrantListMarkdown(grants []dboauth.RevokedGrantDependency) string {
	var b strings.Builder
	for _, grant := range grants {
		fmt.Fprintf(&b, "- %s（`%s`）\n", grant.ClientName, grant.ClientID)
	}
	return b.String()
}

func disabledClientListMarkdown(clients []dboauth.DisabledClientDependency) string {
	var b strings.Builder
	for _, client := range clients {
		fmt.Fprintf(&b, "- %s（`%s`）\n", client.Name, client.ClientID)
	}
	return b.String()
}

func sortedKeys[T any](m map[string][]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
