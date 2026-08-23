# Frontend Component Guide

This directory keeps UI pieces small, reusable, and easy to migrate across
dashboard, admin, and public pages.

## Layers

- `common/`: layout and interaction primitives with no domain API calls.
- `textures/`: texture rendering cards shared by public, dashboard, and admin pages.
- `dashboard/roles/`: role-management components used by `DashboardRoles.vue`.
- `admin/roles/` and `admin/users/`: admin feature UI extracted from page containers.
- `admin/` and `dashboard/`: page-level containers and feature screens.
- `SkinViewer.vue` / `CapeViewer.vue`: rendering widgets used by feature components.

Page-level components should own data loading, routing, notifications, and API
calls. Child components should receive data through props and report user intent
through events.

## Common Components

### `ActionBar`

Use this for page-header buttons and toolbar-style actions.

```vue
<ActionBar full>
  <UiButton variant="gradient-primary">新建</UiButton>
  <el-button>导入</el-button>
</ActionBar>
```

Props:

- `align`: `start | center | end`, defaults to `end`.
- `full`: makes the action row occupy the full header row and wrap cleanly.

Rules:

- Do not hand-roll `.page-header-actions` in feature pages.
- Let `ActionBar` own flex wrapping and Element Plus button margins.
- Page components may set button basis/width via a local class when needed.

### `CardActions`

Use this for buttons at the bottom of entity cards.

```vue
<CardActions>
  <UiButton variant="gradient-danger">删除</UiButton>
  <UiButton variant="soft-warning">清除</UiButton>
</CardActions>
```

Rules:

- Do not duplicate card action flex styles in feature components.
- Buttons automatically share width and wrap on narrow screens.

### `SearchBar`

Use this for search inputs with the standard prefix icon and appended search
button.

```vue
<SearchBar
  v-model="searchQuery"
  placeholder="搜索用户名 / 邮箱"
  @search="handleSearch"
  @clear="handleClearSearch"
/>
```

Rules:

- Keep pagination reset and API calls in the parent page.
- Use local layout classes only for width or placement.
- Do not duplicate `.el-input-group__append` styling in pages.

## Texture Cards

`textures/TextureCard.vue` owns the repeated skin/cape preview card structure:

- static `SkinViewer` / `CapeViewer` preview;
- dark/light preview background;
- optional skin resolution badge;
- default title/type/subtitle block;
- `info` and `actions` slots for page-specific metadata and buttons.

Pages still own `texturesUrl`, loading the resolution map, permissions, API
calls, and dialog state. Use the card anywhere a grid renders a texture entity;
do not duplicate texture-card preview layout or card footer actions in a page.

`textures/TexturePreviewStage.vue` owns the large dialog preview stage for a
single texture. Use it inside `UiViewerLayout` dialogs instead of switching
between `SkinViewer` and `CapeViewer` in page containers.

## Role Management Split

`DashboardRoles.vue` is now the orchestration layer:

- loads and paginates profiles;
- calls profile, Microsoft, and remote-Yggdrasil APIs;
- owns notifications and refresh behavior.

Domain UI pieces live in `dashboard/roles/`:

- `RoleCard.vue`: card preview and card-level actions.
- `RolePreviewDialog.vue`: preview, rename, avatar, clear, and delete actions.
- `CreateRoleDialog.vue`: create-role form.
- `OfficialBindingDialog.vue`: select an existing Microsoft OIDC identity, bind its
  official profile, and explicitly synchronize that profile into a local role.
- `RemoteYggImportDialog.vue`: remote Yggdrasil import flow.

Identity management lives in `dashboard/identities/`, while administrator-managed
OIDC provider configuration lives in `admin/identity/`.

Admin role UI follows the same split in `admin/roles/`:

- `AdminRoleCard.vue`: admin role preview card with owner metadata.
- `AdminRolePreviewDialog.vue`: admin role rename, binding clear, and delete UI.

## Admin User Split

`AdminUserList.vue` remains the container for user search, pagination, avatar
loading, and admin API calls.

UI-only pieces live in `admin/users/`:

- `UserDetailDialog.vue`: user identity panel, profile list, role/permission editor, and danger actions.
- `ResetPasswordDialog.vue`: reset-password form.
- `BanUserDialog.vue`: preset/custom ban duration form.

These components emit intent events such as `grant-role`, `set-permission`, `show-ban`, and
`delete-user`; they should not import admin API modules.

## Migration Checklist

When cleaning another page:

1. Move repeated toolbar/card action markup to `ActionBar` or `CardActions`.
2. Extract entity cards before extracting business logic.
3. Keep API calls in the page or a composable, not in presentational cards.
4. Replace inline styles with component-scoped classes.
5. Run `npm run build` before committing.
