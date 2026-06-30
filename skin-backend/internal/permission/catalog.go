package permission

type Resource struct {
	ID          ResourceID
	Code        string
	Description string
}

type Action struct {
	ID          ActionID
	Code        string
	Description string
}

type Scope struct {
	ID          ScopeID
	Code        string
	ResolverKey string
	Description string
}

type Definition struct {
	ID          ID
	Code        string
	BitIndex    int
	Resource    Resource
	Action      Action
	Scope       Scope
	Description string
}

const (
	ResourceAccount ResourceID = iota + 1
	ResourceAccountPassword
	ResourceUser
	ResourceProfile
	ResourceTexture
	ResourceWardrobe
	ResourceWardrobeEntry
	ResourceNotice
	ResourceSiteSettings
	ResourceSitePublic
	ResourcePermission
	ResourcePermissionRole
	ResourcePermissionAudit
	ResourcePermissionProtected
	ResourceYggdrasilSession
	ResourceYggdrasilServer
	ResourceMicrosoftImport
	ResourceAudit
	ResourceCache
	ResourceInvite
	ResourceHomepageMedia
	ResourceOfficialWhitelist
	ResourceOAuthApp
	ResourceOAuthGrant
	ResourceOAuthToken
	ResourceMinecraftProfile
	ResourceMinecraftTextureProperty
	ResourceMinecraftSession
)

const (
	ActionRead ActionID = iota + 1
	ActionCreate
	ActionUpdate
	ActionDelete
	ActionUpdateMetadata
	ActionUpdateVisibility
	ActionApply
	ActionClear
	ActionAdd
	ActionRemove
	ActionDismiss
	ActionGrant
	ActionRevoke
	ActionManage
	ActionRefresh
	ActionValidate
	ActionInvalidate
	ActionSignout
	ActionJoin
	ActionHasJoined
	ActionStart
	ActionReadProfile
	ActionCreateProfile
	ActionArchive
	ActionBan
	ActionUnban
	ActionReview
	ActionIntrospect
)

const (
	ScopeSelf ScopeID = iota + 1
	ScopeOwned
	ScopeBoundProfile
	ScopePublic
	ScopeAssigned
	ScopeAny
	ScopeSystem
	ScopeServer
)

var Resources = []Resource{
	{ResourceAccount, "account", "账号"},
	{ResourceAccountPassword, "account_password", "账号密码"},
	{ResourceUser, "user", "用户"},
	{ResourceProfile, "profile", "角色"},
	{ResourceTexture, "texture", "材质"},
	{ResourceWardrobe, "wardrobe", "衣柜"},
	{ResourceWardrobeEntry, "wardrobe_entry", "衣柜条目"},
	{ResourceNotice, "notice", "通知"},
	{ResourceSiteSettings, "site_settings", "站点设置"},
	{ResourceSitePublic, "site_public", "公开站点配置"},
	{ResourcePermission, "permission", "权限"},
	{ResourcePermissionRole, "permission_role", "权限角色"},
	{ResourcePermissionAudit, "permission_audit", "权限审计"},
	{ResourcePermissionProtected, "permission_protected", "受保护权限主体"},
	{ResourceYggdrasilSession, "yggdrasil_session", "Yggdrasil 会话"},
	{ResourceYggdrasilServer, "yggdrasil_server", "Yggdrasil 服务器登录"},
	{ResourceMicrosoftImport, "microsoft_import", "Microsoft 正版角色导入"},
	{ResourceAudit, "audit", "审计"},
	{ResourceCache, "cache", "缓存"},
	{ResourceInvite, "invite", "邀请码"},
	{ResourceHomepageMedia, "homepage_media", "首页媒体"},
	{ResourceOfficialWhitelist, "official_whitelist", "官方白名单"},
	{ResourceOAuthApp, "oauth_app", "OAuth 应用"},
	{ResourceOAuthGrant, "oauth_grant", "OAuth 授权"},
	{ResourceOAuthToken, "oauth_token", "OAuth 令牌"},
	{ResourceMinecraftProfile, "minecraft_profile", "Minecraft 角色资料"},
	{ResourceMinecraftTextureProperty, "minecraft_texture_property", "Minecraft 材质属性"},
	{ResourceMinecraftSession, "minecraft_session", "Minecraft 会话能力"},
}

var Actions = []Action{
	{ActionRead, "read", "读取"},
	{ActionCreate, "create", "创建"},
	{ActionUpdate, "update", "修改"},
	{ActionDelete, "delete", "删除"},
	{ActionUpdateMetadata, "update_metadata", "修改元数据"},
	{ActionUpdateVisibility, "update_visibility", "修改可见性"},
	{ActionApply, "apply", "应用"},
	{ActionClear, "clear", "清除"},
	{ActionAdd, "add", "加入"},
	{ActionRemove, "remove", "移除"},
	{ActionDismiss, "dismiss", "忽略"},
	{ActionGrant, "grant", "授予"},
	{ActionRevoke, "revoke", "撤销"},
	{ActionManage, "manage", "管理"},
	{ActionRefresh, "refresh", "刷新"},
	{ActionValidate, "validate", "校验"},
	{ActionInvalidate, "invalidate", "失效"},
	{ActionSignout, "signout", "退出"},
	{ActionJoin, "join", "加入服务器"},
	{ActionHasJoined, "hasjoined", "查询加入结果"},
	{ActionStart, "start", "启动"},
	{ActionReadProfile, "read_profile", "读取角色资料"},
	{ActionCreateProfile, "create_profile", "创建角色"},
	{ActionArchive, "archive", "归档"},
	{ActionBan, "ban", "封禁"},
	{ActionUnban, "unban", "解封"},
	{ActionReview, "review", "审核"},
	{ActionIntrospect, "introspect", "检查"},
}

var Scopes = []Scope{
	{ScopeSelf, "self", "subject_user", "自身账号资源"},
	{ScopeOwned, "owned", "resource_owner", "自有业务资源"},
	{ScopeBoundProfile, "bound_profile", "session_bound_profile", "当前会话绑定角色"},
	{ScopePublic, "public", "resource_public", "公开资源"},
	{ScopeAssigned, "assigned", "resource_assignment", "分配资源"},
	{ScopeAny, "any", "global", "任意资源"},
	{ScopeSystem, "system", "system_actor", "系统任务"},
	{ScopeServer, "server", "client_server", "授权服务器资源"},
}

var Definitions = definitions(
	def(ResourceAccount, ActionRead, ScopeSelf, "读取自己的账号资料"),
	def(ResourceAccount, ActionUpdate, ScopeSelf, "修改自己的账号资料"),
	def(ResourceAccountPassword, ActionUpdate, ScopeSelf, "修改自己的密码"),
	def(ResourceAccount, ActionDelete, ScopeSelf, "注销自己的账号"),
	def(ResourceAccount, ActionBan, ScopeAny, "封禁任意账号"),
	def(ResourceAccount, ActionUnban, ScopeAny, "解除任意账号封禁"),
	def(ResourceAccount, ActionRead, ScopeAny, "读取任意账号资料"),
	def(ResourceAccount, ActionUpdate, ScopeAny, "修改任意账号资料"),
	def(ResourceAccount, ActionDelete, ScopeAny, "删除任意账号"),
	def(ResourceUser, ActionRead, ScopeAny, "管理后台读取用户"),
	def(ResourceUser, ActionUpdate, ScopeAny, "管理后台修改用户"),
	def(ResourceProfile, ActionRead, ScopeOwned, "读取自己的角色"),
	def(ResourceProfile, ActionCreate, ScopeOwned, "创建自己的角色"),
	def(ResourceProfile, ActionUpdate, ScopeOwned, "修改自己的角色"),
	def(ResourceProfile, ActionDelete, ScopeOwned, "删除自己的角色"),
	def(ResourceProfile, ActionRead, ScopeBoundProfile, "读取当前 token 绑定角色"),
	def(ResourceProfile, ActionUpdate, ScopeBoundProfile, "修改当前 token 绑定角色"),
	def(ResourceProfile, ActionRead, ScopePublic, "读取公开角色"),
	def(ResourceProfile, ActionRead, ScopeAny, "管理后台读取任意角色"),
	def(ResourceProfile, ActionUpdate, ScopeAny, "管理后台修改任意角色"),
	def(ResourceProfile, ActionDelete, ScopeAny, "管理后台删除任意角色"),
	def(ResourceTexture, ActionRead, ScopeOwned, "读取自己的材质"),
	def(ResourceTexture, ActionRead, ScopePublic, "读取公开材质"),
	def(ResourceTexture, ActionCreate, ScopeOwned, "上传自己的材质"),
	def(ResourceTexture, ActionUpdateMetadata, ScopeOwned, "修改自己的材质元数据"),
	def(ResourceTexture, ActionUpdateVisibility, ScopeOwned, "修改自己的材质公开状态"),
	def(ResourceTexture, ActionDelete, ScopeOwned, "删除自己的材质"),
	def(ResourceTexture, ActionApply, ScopeOwned, "给自己的角色应用材质"),
	def(ResourceTexture, ActionClear, ScopeOwned, "清除自己角色的材质"),
	def(ResourceTexture, ActionApply, ScopeBoundProfile, "Yggdrasil token 应用材质"),
	def(ResourceTexture, ActionClear, ScopeBoundProfile, "Yggdrasil token 清除材质"),
	def(ResourceTexture, ActionRead, ScopeAssigned, "审核员读取分配材质"),
	def(ResourceTexture, ActionReview, ScopeAssigned, "审核员审核分配材质"),
	def(ResourceTexture, ActionUpdate, ScopeAssigned, "审核员修改审核状态"),
	def(ResourceTexture, ActionDelete, ScopeAssigned, "审核员删除违规材质"),
	def(ResourceTexture, ActionRead, ScopeAny, "管理后台读取任意材质"),
	def(ResourceTexture, ActionUpdateMetadata, ScopeAny, "管理后台修改任意材质元数据"),
	def(ResourceTexture, ActionUpdateVisibility, ScopeAny, "管理后台修改任意材质公开状态"),
	def(ResourceTexture, ActionDelete, ScopeAny, "管理后台删除任意材质"),
	def(ResourceWardrobe, ActionRead, ScopeOwned, "读取自己的衣柜"),
	def(ResourceWardrobeEntry, ActionRead, ScopeOwned, "读取自己的衣柜条目"),
	def(ResourceWardrobeEntry, ActionAdd, ScopeOwned, "加入自己的衣柜"),
	def(ResourceWardrobeEntry, ActionUpdate, ScopeOwned, "修改自己的衣柜条目"),
	def(ResourceWardrobeEntry, ActionRemove, ScopeOwned, "移除自己的衣柜条目"),
	def(ResourceWardrobeEntry, ActionApply, ScopeOwned, "应用自己的衣柜条目"),
	def(ResourceWardrobe, ActionRead, ScopeAny, "管理后台读取任意衣柜"),
	def(ResourceWardrobeEntry, ActionRemove, ScopeAny, "管理后台移除任意衣柜条目"),
	def(ResourceNotice, ActionRead, ScopeOwned, "读取投递给自己的通知"),
	def(ResourceNotice, ActionDismiss, ScopeOwned, "忽略投递给自己的通知"),
	def(ResourceNotice, ActionRead, ScopeAny, "管理后台读取通知"),
	def(ResourceNotice, ActionCreate, ScopeAny, "发布通知"),
	def(ResourceNotice, ActionUpdate, ScopeAny, "替换通知"),
	def(ResourceNotice, ActionDelete, ScopeAny, "删除通知"),
	def(ResourceNotice, ActionDelete, ScopeSystem, "系统删除过期通知"),
	def(ResourceSiteSettings, ActionRead, ScopeAny, "读取站点设置"),
	def(ResourceSiteSettings, ActionUpdate, ScopeAny, "修改站点设置"),
	def(ResourceSitePublic, ActionRead, ScopePublic, "读取公开站点设置"),
	def(ResourceInvite, ActionRead, ScopeAny, "读取邀请码"),
	def(ResourceInvite, ActionCreate, ScopeAny, "创建邀请码"),
	def(ResourceInvite, ActionDelete, ScopeAny, "删除邀请码"),
	def(ResourceHomepageMedia, ActionRead, ScopeAny, "读取首页媒体"),
	def(ResourceHomepageMedia, ActionCreate, ScopeAny, "创建首页媒体"),
	def(ResourceHomepageMedia, ActionUpdate, ScopeAny, "修改首页媒体"),
	def(ResourceHomepageMedia, ActionDelete, ScopeAny, "删除首页媒体"),
	def(ResourceOfficialWhitelist, ActionRead, ScopeAny, "读取官方白名单"),
	def(ResourceOfficialWhitelist, ActionAdd, ScopeAny, "添加官方白名单"),
	def(ResourceOfficialWhitelist, ActionRemove, ScopeAny, "移除官方白名单"),
	def(ResourcePermission, ActionRead, ScopeAny, "读取权限配置"),
	def(ResourcePermissionRole, ActionCreate, ScopeAny, "创建权限角色"),
	def(ResourcePermissionRole, ActionUpdate, ScopeAny, "修改权限角色"),
	def(ResourcePermissionRole, ActionDelete, ScopeAny, "删除权限角色"),
	def(ResourcePermission, ActionGrant, ScopeAny, "授予权限"),
	def(ResourcePermission, ActionRevoke, ScopeAny, "撤销权限"),
	def(ResourcePermissionAudit, ActionRead, ScopeAny, "读取权限审计"),
	def(ResourcePermissionProtected, ActionManage, ScopeAny, "管理受保护权限主体"),
	def(ResourceYggdrasilSession, ActionCreate, ScopeOwned, "创建 Yggdrasil token"),
	def(ResourceYggdrasilSession, ActionRefresh, ScopeOwned, "刷新 Yggdrasil token"),
	def(ResourceYggdrasilSession, ActionValidate, ScopeOwned, "校验 Yggdrasil token"),
	def(ResourceYggdrasilSession, ActionInvalidate, ScopeOwned, "失效 Yggdrasil token"),
	def(ResourceYggdrasilSession, ActionSignout, ScopeOwned, "退出 Yggdrasil 会话"),
	def(ResourceYggdrasilSession, ActionDelete, ScopeSystem, "系统删除过期 Yggdrasil 会话"),
	def(ResourceYggdrasilServer, ActionJoin, ScopeBoundProfile, "加入 Minecraft 服务器"),
	def(ResourceYggdrasilServer, ActionHasJoined, ScopeBoundProfile, "查询服务器加入结果"),
	def(ResourceMicrosoftImport, ActionStart, ScopeOwned, "启动 Microsoft 正版角色导入"),
	def(ResourceMicrosoftImport, ActionReadProfile, ScopeOwned, "读取 Microsoft 角色资料"),
	def(ResourceMicrosoftImport, ActionCreateProfile, ScopeOwned, "导入 Microsoft 角色"),
	def(ResourceAudit, ActionRead, ScopeAny, "读取审计日志"),
	def(ResourceAudit, ActionArchive, ScopeSystem, "系统归档审计日志"),
	def(ResourceCache, ActionInvalidate, ScopeSystem, "系统失效缓存"),
	def(ResourceCache, ActionInvalidate, ScopeAny, "管理员失效缓存"),
	def(ResourceOAuthApp, ActionRead, ScopeOwned, "读取自己的 OAuth 应用"),
	def(ResourceOAuthApp, ActionCreate, ScopeOwned, "创建自己的 OAuth 应用"),
	def(ResourceOAuthApp, ActionUpdate, ScopeOwned, "修改自己的 OAuth 应用"),
	def(ResourceOAuthApp, ActionDelete, ScopeOwned, "删除自己的 OAuth 应用"),
	def(ResourceOAuthApp, ActionRead, ScopeAny, "管理后台读取 OAuth 应用"),
	def(ResourceOAuthApp, ActionUpdate, ScopeAny, "管理后台修改 OAuth 应用"),
	def(ResourceOAuthGrant, ActionRead, ScopeOwned, "读取授予自己的 OAuth 授权"),
	def(ResourceOAuthGrant, ActionRevoke, ScopeOwned, "撤销授予自己的 OAuth 授权"),
	def(ResourceOAuthGrant, ActionRead, ScopeAny, "管理后台读取 OAuth 授权"),
	def(ResourceOAuthGrant, ActionRevoke, ScopeAny, "管理后台撤销 OAuth 授权"),
	def(ResourceOAuthToken, ActionRevoke, ScopeOwned, "撤销自己的 OAuth 令牌"),
	def(ResourceOAuthToken, ActionIntrospect, ScopeAny, "检查 OAuth 令牌"),
	def(ResourceMinecraftProfile, ActionRead, ScopePublic, "读取公开 Minecraft 角色资料"),
	def(ResourceMinecraftTextureProperty, ActionRead, ScopePublic, "读取公开 Minecraft 材质属性"),
	def(ResourceMinecraftSession, ActionHasJoined, ScopeServer, "查询授权服务器的加入结果"),
	def(ResourceOAuthApp, ActionDelete, ScopeAny, "管理后台删除 OAuth 应用"),
)

type defInput struct {
	resource    ResourceID
	action      ActionID
	scope       ScopeID
	description string
}

func def(resource ResourceID, action ActionID, scope ScopeID, description string) defInput {
	return defInput{resource: resource, action: action, scope: scope, description: description}
}

func definitions(inputs ...defInput) []Definition {
	resources := resourceMap()
	actions := actionMap()
	scopes := scopeMap()
	out := make([]Definition, 0, len(inputs))
	for i, input := range inputs {
		resource := resources[input.resource]
		action := actions[input.action]
		scope := scopes[input.scope]
		id := MustComposeID(input.resource, input.action, input.scope)
		out = append(out, Definition{
			ID:          id,
			Code:        resource.Code + "." + action.Code + "." + scope.Code,
			BitIndex:    i,
			Resource:    resource,
			Action:      action,
			Scope:       scope,
			Description: input.description,
		})
	}
	return out
}

func resourceMap() map[ResourceID]Resource {
	out := make(map[ResourceID]Resource, len(Resources))
	for _, item := range Resources {
		out[item.ID] = item
	}
	return out
}

func actionMap() map[ActionID]Action {
	out := make(map[ActionID]Action, len(Actions))
	for _, item := range Actions {
		out[item.ID] = item
	}
	return out
}

func scopeMap() map[ScopeID]Scope {
	out := make(map[ScopeID]Scope, len(Scopes))
	for _, item := range Scopes {
		out[item.ID] = item
	}
	return out
}
