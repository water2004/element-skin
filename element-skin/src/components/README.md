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

### `UiButton`

Use `variant="primary | secondary | tertiary"` for the action hierarchy and
`tone="neutral | primary | success | warning | danger"` for its meaning. Defaults
are `secondary` and `neutral`. A primary neutral action uses the primary color;
secondary colored actions are outlined; tertiary actions use text styling.

```vue
<UiButton variant="primary" :icon="Plus">新建</UiButton>
<UiButton tone="danger">删除</UiButton>
<UiButton variant="tertiary" :icon="Refresh" icon-only aria-label="刷新" />
```

`size`, `icon`, `iconOnly`, `loading`, and `disabled` are independent props. Icon-only
buttons require an accessible name such as `aria-label` and do not render a label
wrapper. Loading keeps the normal content in the layout and overlays a spinner;
loading and disabled buttons do not emit clicks. Native attributes, including
`native-type`, `tag="a"`, `href`, and event modifiers, are forwarded. Disabled or
loading links have no active `href`.

### `UiCard`

`shadow="never | hover | always"` defaults to `never` and controls the actual
shadow. `hoverable` is an explicit motion opt-in for interactive cards and does
not override `shadow="never"`. Cards containing only independent buttons should
not be marked `hoverable`.

Use `padding="none | compact | default | spacious"` for the card body
(0 / 16 / 20 / 24px). Avoid adding padding to both the card root and its body.

### `UiDialog`

Use the existing `form`, `wide-form`, or `viewer` variant for layout. `busy` blocks
user dismissal during a critical operation; ordinary data loading need not block
closing. `dirty` asks whether to discard unsaved edits on mask or Escape dismissal.
Cancel and the close icon explicitly discard without another confirmation. The page owns the draft,
baseline and actual reset; the dialog does not write data or store drafts.

```vue
<UiDialog v-model="visible" :busy="saving" :dirty="hasChanges" @closed="resetForm">
  <!-- form -->
  <template #footer="{ requestClose }">
    <UiButton :disabled="saving" @click="requestClose">取消</UiButton>
    <UiButton variant="primary" :loading="saving" @click="save">保存</UiButton>
  </template>
</UiDialog>
```

Mask, Escape, the close icon and `requestClose` share busy protection. `requestClose`
is an explicit discard; `requestDismiss` (also exposed on the instance) checks dirty
state before dismissal. Native
`close-on-click-modal`, `close-on-press-escape`, `before-close` and dialog events
remain available. Header slots receive the native title IDs and explicit `close`;
the default/footer slots and component instance expose `requestClose`. Repeated
close requests share one discard confirmation. A delayed confirmation from a
previous opening cannot close a new opening or emit after unmount.

Parent `visible = false` means an authorized programmatic transition (save
success or next/back), so it intentionally bypasses discard checks. User cancel
buttons must call `requestClose`. `closed` is emitted after leave only if the
dialog is still closed, preventing old transitions from clearing new input.
Sensitive forms also clear credentials on unmount. Parent-page drafts, such as
homepage media, remain in their owner when the local editor closes; clearly
label the parent save step instead of applying independent-form discard rules.

### `PageHeader`

Provide `title`, optional `subtitle`, and `icon` / `actions` slots. `level` selects
the heading level (1, 2, or 3; default 1). Long text wraps. The actions slot uses
`ActionBar`; optional `actionsLabel` supplies its accessible group name. The
component owns its styles and does not depend on AppLayout's legacy header CSS.

### `ActionBar`

Use this for page-header buttons and toolbar-style actions.

```vue
<ActionBar full>
  <UiButton variant="primary">新建</UiButton>
  <UiButton>导入</UiButton>
</ActionBar>
```

Props:

- `align`: `start | center | end`, defaults to `end`.
- `mobileAlign`: `start | center | end`, defaults to `start` at 768px and below.
- `label`: optional accessible group name.
- `full`: makes the action row occupy the full header row and wrap cleanly.

Rules:

- Do not hand-roll `.page-header-actions` in feature pages.
- Let `ActionBar` own flex wrapping and Element Plus button margins.
- Page components may set button basis/width via a local class when needed.

### `CardActions`

Use this for buttons at the bottom of entity cards.

```vue
<CardActions label="角色操作">
  <UiButton tone="danger">删除</UiButton>
  <UiButton variant="tertiary" tone="warning">清除皮肤</UiButton>
</CardActions>
```

Rules:

- Do not duplicate card action flex styles in feature components.
- Buttons share width and wrap according to the available card width, including
  narrow cards on a wide screen. `label` optionally names the action group.

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
- `loading` blocks repeated searches while preserving the query; `disabled`
  disables input and search. `label` supplies the input's accessible name.
- Click and Enter emit one `search`; IME composition does not submit. Clearing
  updates the model and emits `clear`, without implicitly starting a search.

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

### Unsaved page changes

The root reserves a stable scrollbar gutter. The shared Element Plus overrides
neutralize body width compensation for popup, message-box and image-viewer lock
classes together. Keep overflow locking and native cleanup; do not fix a single
dialog by changing the page container width or disabling scroll protection.

Full-page forms use `useUnsavedChangesGuard({ dirty, busy })` in the route-owned
page. `dirty` must reflect the actual draft and its last acknowledged baseline;
`busy` covers writes, not ordinary reads. The helper confirms router navigation
and object-param changes, shares a pending confirmation, blocks navigation during
writes, and attaches `beforeunload` only while protection is needed. Same-object
query/anchor navigation does not discard a draft. Pages that reload data must
call `confirmReload()` before replacing their fields. A successful confirmation
does not clear fields: navigation may still fail in another guard.

Browser refresh, tab/window close, and address-bar navigation use the browser's
native warning. Custom text and asynchronous saving are unavailable there. Do not
persist credentials or automatically submit on unload.

Homepage media is one parent-owned configuration draft: field changes, enabled
state, and ordering wait for “保存配置”; the local editor's “返回配置”, close icon,
mask and Escape preserve it. “放弃更改” discards only changes not acknowledged by
the server. Upload/delete are immediate resource commands and require handling
the draft first; partial save progress must advance the acknowledged baseline.

### Motion

Use `animate-fade-in` for a content group and `animate-card-slide` for individual
cards. On card-list pages, animate the heading and toolbar together, as a sibling
of the list; do not animate an ancestor of the cards. Cards enter when mounted,
including after a delayed response. Put `--motion-card-count` on the grid and
provide each card's delay index: the interval is at most 30ms, distributed across
the batch within 120ms so later cards do not share one capped delay. The
shared keyframe uses `translate`, leaving `transform` available for explicit
interaction; backwards fill covers delays without retaining a transform after
entry. Motion timings and distances come from the shared tokens; Tailwind's
default transitions and Element Plus durations use those tokens as well.

The profile display avatar intentionally retains its hover scale and rotation
when hover is supported and reduced motion is not requested. Keep this exception
local to the profile avatar; ordinary form controls retain paint-only feedback.

Home copy and actions start from one `is-entered` state after the session is ready
and two animation frames. Do not add an independent appear transition to buttons
or fade their glass ancestor. Failed sessions still become ready; avatars and
background media do not gate entrance. Reduced motion keeps content visible,
removes entrance delay/displacement, and preserves loading and sorting feedback.

### Existing component cleanup

When cleaning another page:

1. Move repeated toolbar/card action markup to `ActionBar` or `CardActions`.
2. Extract entity cards before extracting business logic.
3. Keep API calls in the page or a composable, not in presentational cards.
4. Replace inline styles with component-scoped classes.
5. Run `npm run build` before committing.
