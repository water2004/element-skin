package oauth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/util"
)

const (
	reviewNoticeTTL      = 30 * 24 * time.Hour
	maxReviewReasonRunes = 500
)

func (s Service) notifyAdminsClientSubmitted(ctx context.Context, createdBy string, client model.OAuthClient) error {
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, noticesvc.CreateInput{
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
	}, createdBy)
	return err
}

func (s Service) notifyOwnerReviewResult(ctx context.Context, createdBy string, client model.OAuthClient, status, reason string) error {
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
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, noticesvc.CreateInput{
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
	}, createdBy)
	return err
}

func validateReviewReason(status, reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if status == StatusRejected || status == StatusDisabled {
		if reason == "" {
			return "", util.HTTPError{Status: http.StatusBadRequest, Detail: "reason is required"}
		}
	}
	if len([]rune(reason)) > maxReviewReasonRunes {
		return "", util.HTTPError{Status: http.StatusBadRequest, Detail: "reason too long"}
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
