package httpapi

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (r *Router) Capabilities(w http.ResponseWriter, req *http.Request) {
	apiURL := strings.TrimRight(r.cfg.APIURL, "/")
	if apiURL == "" {
		apiURL = strings.TrimRight(r.cfg.SiteURL, "/")
	}
	util.JSON(w, http.StatusOK, map[string]any{
		"api_version": "v1",
		"site_name":   "Element Skin",
		"site_url":    strings.TrimRight(r.cfg.SiteURL, "/"),
		"api_url":     apiURL,
		"features": map[string]bool{
			"skin_library":      true,
			"oauth":             true,
			"device_code":       true,
			"minecraft_api":     true,
			"microsoft_import":  true,
			"remote_ygg_import": true,
		},
		"texture_types": []string{"skin", "cape"},
		"skin_models":   []string{"default", "slim"},
	})
}

func (r *Router) PermissionCatalog(w http.ResponseWriter, req *http.Request) {
	permissions := make([]map[string]any, 0, len(permission.Definitions))
	for _, def := range permission.Definitions {
		permissions = append(permissions, map[string]any{
			"id":                   uint64(def.ID),
			"code":                 def.Code,
			"description":          def.Description,
			"resource":             def.Resource.Code,
			"resource_description": def.Resource.Description,
			"action":               def.Action.Code,
			"action_description":   def.Action.Description,
			"scope":                def.Scope.Code,
			"scope_description":    def.Scope.Description,
			"resolver_key":         def.Scope.ResolverKey,
			"delegable":            def.Scope.ID != permission.ScopeSystem,
			"admin_delegable":      def.Scope.ID == permission.ScopeAny,
			"protected":            def.Resource.ID == permission.ResourcePermissionProtected,
		})
	}
	roles := make([]map[string]any, 0, len(permission.Roles))
	for _, role := range permission.Roles {
		codes := make([]string, 0, len(role.Permissions))
		for _, def := range role.Permissions {
			codes = append(codes, def.Code)
		}
		roles = append(roles, map[string]any{
			"id":          role.ID,
			"name":        role.Name,
			"description": role.Description,
			"system_role": role.SystemRole,
			"protected":   role.Protected,
			"permissions": codes,
		})
	}
	util.JSON(w, http.StatusOK, map[string]any{
		"permissions": permissions,
		"roles":       roles,
	})
}
