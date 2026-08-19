package oauth

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"element-skin/backend/internal/util"
)

func TestValidateReviewReasonExactRules(t *testing.T) {
	trimmed, err := validateReviewReason(StatusRejected, "  需要补充隐私说明  ")
	if err != nil || trimmed != "需要补充隐私说明" {
		t.Fatalf("trimmed rejected reason=%q err=%v; want exact reason and nil", trimmed, err)
	}

	empty, err := validateReviewReason(StatusRejected, "   ")
	if empty != "" || !isInternalHTTPError(err, http.StatusBadRequest, "audit_reason.validate.required") {
		t.Fatalf("empty rejected reason=%q err=%#v; want exact required error", empty, err)
	}

	empty, err = validateReviewReason(StatusDisabled, "")
	if empty != "" || !isInternalHTTPError(err, http.StatusBadRequest, "audit_reason.validate.required") {
		t.Fatalf("empty disabled reason=%q err=%#v; want exact required error", empty, err)
	}

	long, err := validateReviewReason(StatusActive, strings.Repeat("理", maxReviewReasonRunes+1))
	if long != "" || !isInternalHTTPError(err, http.StatusBadRequest, "audit_reason.validate.too_long") {
		t.Fatalf("long active reason len=%d err=%#v; want exact too-long error", len([]rune(long)), err)
	}

	optional, err := validateReviewReason(StatusActive, "  ")
	if err != nil || optional != "" {
		t.Fatalf("optional active reason=%q err=%v; want empty reason and nil", optional, err)
	}
}

func isInternalHTTPError(err error, status int, classification string) bool {
	var target util.HTTPError
	return errors.As(err, &target) && target.Status == status && target.Error() == classification
}

func TestReviewNotificationLabelsAndTruncationExactly(t *testing.T) {
	labels := map[string]string{
		StatusActive:   "已通过",
		StatusRejected: "已驳回",
		StatusDisabled: "已停用",
		"archived":     "archived",
	}
	for status, want := range labels {
		if got := reviewStatusLabel(status); got != want {
			t.Fatalf("reviewStatusLabel(%q)=%q; want %q", status, got, want)
		}
	}

	textCases := []struct {
		value string
		max   int
		want  string
	}{
		{value: "  正好三  ", max: 3, want: "正好三"},
		{value: "无需截断", max: 4, want: "无需截断"},
		{value: "需要被截断", max: 4, want: "需要被…"},
		{value: "边界", max: 1, want: "边"},
		{value: "边界", max: 0, want: ""},
	}
	for _, tc := range textCases {
		if got := fitNoticeText(tc.value, tc.max); got != tc.want {
			t.Fatalf("fitNoticeText(%q,%d)=%q; want %q", tc.value, tc.max, got, tc.want)
		}
	}

	if got := fitNoticeTitle("第三方应用审核通过：", strings.Repeat("长", 80)); len([]rune(got)) != 80 || !strings.HasSuffix(got, "…") {
		t.Fatalf("fitNoticeTitle length/suffix mismatch: len=%d title=%q", len([]rune(got)), got)
	}
}
