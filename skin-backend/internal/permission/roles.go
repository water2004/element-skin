package permission

type Role struct {
	ID          string
	Name        string
	Description string
	SystemRole  bool
	Protected   bool
	Permissions []Definition
}

const (
	RoleUser              = "user"
	RoleModerator         = "moderator"
	RoleAdmin             = "admin"
	RoleSuperAdmin        = "super_admin"
	RoleSystemMaintenance = "system_maintenance"

	SessionKindWeb        = "web"
	SessionKindYggdrasil  = "yggdrasil"
	SessionKindSystem     = "system"
	SessionKindDelegated  = "delegated"
	EntrypointDashboard   = "dashboard"
	EntrypointAdmin       = "admin"
	EntrypointYggdrasil   = "yggdrasil"
	EntrypointMaintenance = "maintenance"
)

var Roles = []Role{
	{
		ID:          RoleUser,
		Name:        "用户",
		Description: "普通站点用户",
		SystemRole:  true,
		Permissions: definitionsByCodes(
			"account.read.self",
			"account.update.self",
			"account_password.update.self",
			"account.delete.self",
			"profile.read.owned",
			"profile.create.owned",
			"profile.update.owned",
			"profile.delete.owned",
			"profile.read.bound_profile",
			"profile.update.bound_profile",
			"profile.read.public",
			"texture.read.owned",
			"texture.read.public",
			"texture.create.owned",
			"texture.update_metadata.owned",
			"texture.update_visibility.owned",
			"texture.delete.owned",
			"texture.apply.owned",
			"texture.clear.owned",
			"texture.apply.bound_profile",
			"texture.clear.bound_profile",
			"wardrobe.read.owned",
			"wardrobe_entry.read.owned",
			"wardrobe_entry.add.owned",
			"wardrobe_entry.update.owned",
			"wardrobe_entry.remove.owned",
			"wardrobe_entry.apply.owned",
			"notice.read.owned",
			"notice.dismiss.owned",
			"site_public.read.public",
			"yggdrasil_session.create.owned",
			"yggdrasil_session.refresh.owned",
			"yggdrasil_session.validate.owned",
			"yggdrasil_session.invalidate.owned",
			"yggdrasil_session.signout.owned",
			"yggdrasil_server.join.bound_profile",
			"yggdrasil_server.hasjoined.bound_profile",
			"microsoft_import.start.owned",
			"microsoft_import.read_profile.owned",
			"microsoft_import.create_profile.owned",
		),
	},
	{
		ID:          RoleModerator,
		Name:        "审核员",
		Description: "材质审核与违规内容处理",
		SystemRole:  true,
		Permissions: definitionsByCodes(
			"texture.read.assigned",
			"texture.review.assigned",
			"texture.update.assigned",
			"texture.delete.assigned",
		),
	},
	{
		ID:          RoleAdmin,
		Name:        "管理员",
		Description: "站点运营管理",
		SystemRole:  true,
		Permissions: definitionsByCodes(
			"account.ban.any",
			"account.unban.any",
			"account.read.any",
			"account.update.any",
			"account.delete.any",
			"user.read.any",
			"user.update.any",
			"profile.read.any",
			"profile.update.any",
			"profile.delete.any",
			"texture.read.any",
			"texture.update_metadata.any",
			"texture.update_visibility.any",
			"texture.delete.any",
			"wardrobe.read.any",
			"wardrobe_entry.remove.any",
			"notice.read.any",
			"notice.create.any",
			"notice.update.any",
			"notice.delete.any",
			"site_settings.read.any",
			"site_settings.update.any",
			"invite.read.any",
			"invite.create.any",
			"invite.delete.any",
			"homepage_media.read.any",
			"homepage_media.create.any",
			"homepage_media.update.any",
			"homepage_media.delete.any",
			"official_whitelist.read.any",
			"official_whitelist.add.any",
			"official_whitelist.remove.any",
			"permission.read.any",
			"permission.grant.any",
			"permission.revoke.any",
			"permission_audit.read.any",
			"audit.read.any",
			"cache.invalidate.any",
		),
	},
	{
		ID:          RoleSuperAdmin,
		Name:        "超级管理员",
		Description: "拥有全部非系统权限与受保护权限管理能力",
		SystemRole:  true,
		Protected:   true,
		Permissions: nonSystemDefinitions(),
	},
	{
		ID:          RoleSystemMaintenance,
		Name:        "系统维护",
		Description: "系统后台维护任务",
		SystemRole:  true,
		Protected:   true,
		Permissions: systemDefinitions(),
	},
}

type SessionPolicy struct {
	SessionKind string
	Entrypoint  string
	Permissions []Definition
}

var SessionPolicies = []SessionPolicy{
	{SessionKind: SessionKindWeb, Entrypoint: EntrypointDashboard, Permissions: nonSystemDefinitions()},
	{SessionKind: SessionKindWeb, Entrypoint: EntrypointAdmin, Permissions: nonSystemDefinitions()},
	{
		SessionKind: SessionKindYggdrasil,
		Entrypoint:  EntrypointYggdrasil,
		Permissions: definitionsByCodes(
			"profile.read.bound_profile",
			"profile.update.bound_profile",
			"texture.apply.bound_profile",
			"texture.clear.bound_profile",
			"yggdrasil_session.create.owned",
			"yggdrasil_session.refresh.owned",
			"yggdrasil_session.validate.owned",
			"yggdrasil_session.invalidate.owned",
			"yggdrasil_session.signout.owned",
			"yggdrasil_server.join.bound_profile",
			"yggdrasil_server.hasjoined.bound_profile",
		),
	},
	{SessionKind: SessionKindSystem, Entrypoint: EntrypointMaintenance, Permissions: systemDefinitions()},
}

func DefinitionByCode(code string) (Definition, bool) {
	for _, def := range Definitions {
		if def.Code == code {
			return def, true
		}
	}
	return Definition{}, false
}

func MustDefinitionByCode(code string) Definition {
	def, ok := DefinitionByCode(code)
	if !ok {
		panic("unknown permission code: " + code)
	}
	return def
}

func definitionsByCodes(codes ...string) []Definition {
	out := make([]Definition, 0, len(codes))
	for _, code := range codes {
		out = append(out, MustDefinitionByCode(code))
	}
	return out
}

func nonSystemDefinitions() []Definition {
	out := make([]Definition, 0, len(Definitions))
	for _, def := range Definitions {
		if def.Scope.ID != ScopeSystem {
			out = append(out, def)
		}
	}
	return out
}

func systemDefinitions() []Definition {
	out := make([]Definition, 0, len(Definitions))
	for _, def := range Definitions {
		if def.Scope.ID == ScopeSystem {
			out = append(out, def)
		}
	}
	return out
}
