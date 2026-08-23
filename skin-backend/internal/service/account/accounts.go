package account

import (
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	emailpolicysvc "element-skin/backend/internal/service/emailpolicy"
	verificationsvc "element-skin/backend/internal/service/verification"
)

var (
	manageProtectedPermission = permission.MustDefinitionByCode("permission_protected.manage.any")
	permissionGrantAny        = permission.MustDefinitionByCode("permission.grant.any")
	permissionRevokeAny       = permission.MustDefinitionByCode("permission.revoke.any")
	accountBanPermission      = permission.MustDefinitionByCode("account.ban.any")
	accountUnbanPermission    = permission.MustDefinitionByCode("account.unban.any")
	accountDeletePermission   = permission.MustDefinitionByCode("account.delete.any")
	accountUpdatePermission   = permission.MustDefinitionByCode("account.update.any")
	userReadAnyPermission     = permission.MustDefinitionByCode("user.read.any")
	accountReadAnyPermission  = permission.MustDefinitionByCode("account.read.any")
)

type AccountService struct {
	DB           *database.DB
	Redis        redisstore.Store
	Verification verificationsvc.Service
	EmailPolicy  emailpolicysvc.Service
}

type BanUserInput struct {
	BannedUntil int64
	Reason      string
}

type ResetPasswordInput struct {
	UserID      string
	NewPassword string
}
