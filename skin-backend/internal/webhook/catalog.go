package webhook

import "sort"

type Definition struct {
	Type                      string   `json:"type"`
	Description               string   `json:"description"`
	RequiredPermissions       []string `json:"required_permissions"`
	OwnedPermissionCode       string   `json:"-"`
	ApplicationPermissionCode string   `json:"-"`
	TargetClient              bool     `json:"-"`
}

var Definitions = []Definition{
	definition("oauth_grant.created", "用户向应用授予访问权限", "oauth_grant.read.owned", "", true),
	definition("oauth_grant.updated", "用户更新授予应用的访问权限", "oauth_grant.read.owned", "", true),
	definition("oauth_grant.revoked", "用户撤销授予应用的访问权限", "oauth_grant.read.owned", "", true),
	definition("profile.created", "用户创建角色", "profile.read.owned", "profile.read.any", false),
	definition("profile.updated", "用户角色发生变化", "profile.read.owned", "profile.read.any", false),
	definition("profile.deleted", "用户删除角色", "profile.read.owned", "profile.read.any", false),
	definition("texture.created", "用户创建材质记录", "texture.read.owned", "texture.read.any", false),
	definition("texture.updated", "用户材质记录发生变化", "texture.read.owned", "texture.read.any", false),
	definition("texture.deleted", "用户删除材质记录", "texture.read.owned", "texture.read.any", false),
}

var definitionsByType = func() map[string]Definition {
	out := make(map[string]Definition, len(Definitions))
	for _, item := range Definitions {
		out[item.Type] = item
	}
	return out
}()

func DefinitionByType(eventType string) (Definition, bool) {
	item, ok := definitionsByType[eventType]
	return item, ok
}

func Types() []string {
	out := make([]string, 0, len(Definitions))
	for _, item := range Definitions {
		out = append(out, item.Type)
	}
	sort.Strings(out)
	return out
}

func definition(eventType, description, ownedPermission, applicationPermission string, targetClient bool) Definition {
	permissions := []string{ownedPermission}
	if applicationPermission != "" {
		permissions = append(permissions, applicationPermission)
	}
	sort.Strings(permissions)
	return Definition{
		Type:                      eventType,
		Description:               description,
		RequiredPermissions:       permissions,
		OwnedPermissionCode:       ownedPermission,
		ApplicationPermissionCode: applicationPermission,
		TargetClient:              targetClient,
	}
}
