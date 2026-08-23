package imports

import (
	"context"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s ImportService) ImportProfiles(ctx context.Context, actor permission.Actor, profiles []map[string]string, fetch func(context.Context, string) ([]TextureAsset, error)) map[string]any {
	var items []map[string]any
	var failed []map[string]any
	for _, p := range profiles {
		id := p["profile_id"]
		name := p["profile_name"]
		if id == "" || name == "" {
			failed = append(failed, batchFailure(id, name, util.ErrorBody{Object: "profile", Operation: "import", Reason: "required"}))
			continue
		}
		assets, err := fetch(ctx, id)
		if err != nil {
			failed = append(failed, batchFailure(id, name, util.ErrorBody{Object: "remote_profile", Operation: "fetch", Reason: "unavailable"}))
			continue
		}
		res, err := s.ImportProfile(ctx, actor, id, name, assets)
		if err != nil {
			failure := util.ErrorBody{Object: "profile", Operation: "import", Reason: "failed"}
			if he, ok := err.(util.HTTPError); ok {
				failure = util.ErrorBody{Object: he.Object, Operation: he.Operation, Reason: he.Reason, Params: he.Params}
			}
			failed = append(failed, batchFailure(id, name, failure))
			continue
		}
		items = append(items, res["profile"].(map[string]any))
	}
	return map[string]any{
		"success_count": len(items),
		"failure_count": len(failed),
		"items":         items,
		"failed":        failed,
	}
}

func batchFailure(profileID, profileName string, failure util.ErrorBody) map[string]any {
	return map[string]any{
		"profile_id":   profileID,
		"profile_name": profileName,
		"error":        failure,
	}
}
