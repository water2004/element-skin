package notice

import (
	"context"
	"net/http"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) Create(ctx context.Context, actor permission.Actor, input CreateInput) (*model.Notice, error) {
	if err := requireCreatePermission(actor, input); err != nil {
		return nil, err
	}
	notice, err := noticeFromCreate(input, actorCreatedBy(actor))
	if err != nil {
		return nil, err
	}
	targets, err := normalizedTargetUserIDs(input.TargetUserIDs, notice.Audience)
	if err != nil {
		return nil, err
	}
	if notice.Audience == AudienceTargeted {
		err = s.DB.Notices.CreateWithTargets(ctx, notice, targets)
	} else {
		err = s.DB.Notices.Create(ctx, notice)
	}
	if err != nil {
		return nil, err
	}
	return &notice, nil
}

func (s Service) Patch(ctx context.Context, actor permission.Actor, id string, input PatchInput) (*model.Notice, error) {
	if err := requirePermission(actor, noticeUpdatePermission); err != nil {
		return nil, err
	}
	existing, err := s.DB.Notices.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Object: "notice", Operation: "resolve", Reason: "not_found"}
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
	updated.CreatedBy = actorCreatedBy(actor)
	if err := validateNotice(updated); err != nil {
		return nil, err
	}
	if updated.Audience == AudienceTargeted && existing.Audience != AudienceTargeted {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "notice_target", Operation: "validate", Reason: "required"}
	}
	ok, err := s.DB.Notices.Replace(ctx, id, updated)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, util.HTTPError{Status: http.StatusNotFound, Object: "notice", Operation: "resolve", Reason: "not_found"}
	}
	return &updated, nil
}

func (s Service) Delete(ctx context.Context, actor permission.Actor, id string) error {
	if err := requirePermission(actor, noticeDeletePermission); err != nil {
		return err
	}
	ok, err := s.DB.Notices.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Object: "notice", Operation: "resolve", Reason: "not_found"}
	}
	return nil
}
