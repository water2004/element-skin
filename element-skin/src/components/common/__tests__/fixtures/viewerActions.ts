import type { Profile, Texture } from '@/api/types'

export const viewerProfile: Profile = {
  id: 'viewer-profile',
  name: 'Player',
  model: 'default',
  skin_hash: 'viewer-skin',
  cape_hash: 'viewer-cape',
}
export const viewerTexture: Texture = {
  hash: 'viewer-skin',
  type: 'skin',
  model: 'default',
  note: 'Ocean',
  name: 'Ocean',
  is_public: 0,
  uploader_name: 'Uploader',
  uploader_display_name: 'Uploader',
}
export const viewerReadCases = [
  {
    page: 'library',
    permission: 'texture.read.public',
    title: 'Ocean',
    forbidden: ['添加到衣柜', '添加到我的衣柜'],
  },
  {
    page: 'wardrobe',
    permission: 'texture.read.owned',
    title: 'Ocean',
    forbidden: ['修改材质备注', '确定', '删除纹理', '应用到角色'],
  },
  {
    page: 'roles',
    permission: 'profile.read.owned',
    title: 'Player',
    forbidden: ['修改角色名称', '用作头像', '清除皮肤', '清除披风', '删除此角色'],
  },
  {
    page: 'adminRoles',
    permission: 'profile.read.any',
    title: 'Player',
    forbidden: ['清除皮肤', '清除披风', '删除角色'],
  },
  {
    page: 'adminTextures',
    permission: 'texture.read.any',
    title: 'Ocean',
    forbidden: ['强制下架'],
  },
] as const

export const wardrobeDeleteWarning =
  '当前删除规则：若你是皮肤库记录的原上传者，将删除该库条目及所有用户对应的衣柜记录；否则只移除你自己的衣柜记录。角色绑定、头像和材质文件会保留。当前页面无法确认原上传者身份，请确认可能的影响后再继续。'
export const adminDeleteWarning =
  '强制下架将删除此材质的皮肤库条目及所有用户对应的衣柜记录。已有角色绑定、头像和材质文件会保留，此操作不会停用已应用的材质。确定继续？'
