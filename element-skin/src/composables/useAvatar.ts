import { ref } from 'vue'
import * as skinview3d from 'skinview3d'
import { patchMe } from '@/api/me'
import { appStorage } from '@/utils/storage'

type SkinModel = 'default' | 'slim'

// Global reactive state to ensure sync across components
const currentAvatarImg = ref<string | null>(null)
const avatarHash = ref<string | null>(null)

// Sequential generation queue — ensures only ONE WebGL context exists at a time
let _generationQueue: Promise<unknown> = Promise.resolve()

// Low-level generator using a single WebGL instance. Callers should use getAvatarForHash().
function _doGenerateAvatar(hash: string, model: SkinModel = 'default'): Promise<string | null> {
  return new Promise((resolve) => {
    const canvas = document.createElement('canvas')
    const viewer = new skinview3d.SkinViewer({
      canvas,
      width: 256,
      height: 256,
      model,
      preserveDrawingBuffer: true,
    })

    // Hide body parts
    const skin = viewer.playerObject.skin
    if (skin.body) skin.body.visible = false
    if (skin.leftArm) skin.leftArm.visible = false
    if (skin.rightArm) skin.rightArm.visible = false
    if (skin.leftLeg) skin.leftLeg.visible = false
    if (skin.rightLeg) skin.rightLeg.visible = false
    if (viewer.playerObject.cape) viewer.playerObject.cape.visible = false
    if (viewer.playerObject.elytra) viewer.playerObject.elytra.visible = false

    // Adjust camera to perfectly frame the head
    viewer.playerWrapper.position.y = -12
    viewer.playerWrapper.rotation.y = 0
    viewer.playerWrapper.rotation.x = 0
    viewer.zoom = 4.0
    viewer.autoRotate = false

    const base = import.meta.env.BASE_URL
    const textureUrl = `${base}static/textures/${hash}.png`.replace(/\/+/g, '/')
    viewer
      .loadSkin(textureUrl)
      .then(() => {
        viewer.render()
        const base64 = canvas.toDataURL()
        appStorage.avatar.set(hash, base64)
        viewer.dispose()
        resolve(base64)
      })
      .catch((e) => {
        console.error('Failed to generate avatar for hash', hash, e)
        viewer.dispose()
        resolve(null)
      })
  })
}

/**
 * Get avatar image for any texture hash. Returns cached base64 from app storage
 * instantly, or generates sequentially (WebGL instances serialized via a promise queue).
 */
export function getAvatarForHash(hash: string | null | undefined, model: SkinModel = 'default'): Promise<string | null> {
  if (!hash) return Promise.resolve(null)

  const cached = appStorage.avatar.get(hash)
  if (cached) return Promise.resolve(cached)

  const task = _generationQueue.then(() => _doGenerateAvatar(hash, model))
  _generationQueue = task.catch(() => {}) // swallow errors to keep queue alive
  return task
}

export function useAvatar() {
  async function initializeAvatar(hash: string | null | undefined, model: SkinModel = 'default'): Promise<void> {
    if (!hash) {
      currentAvatarImg.value = null
      avatarHash.value = null
      return
    }

    avatarHash.value = hash
    const cached = appStorage.avatar.get(hash)
    if (cached) {
      currentAvatarImg.value = cached
    } else {
      currentAvatarImg.value = await getAvatarForHash(hash, model)
    }
  }

  async function setAvatar(hash: string | null, model: SkinModel = 'default'): Promise<boolean> {
    await patchMe({ avatar_hash: hash })
    await initializeAvatar(hash, model)
    window.dispatchEvent(new CustomEvent('avatar-changed', { detail: hash }))
    return true
  }

  return {
    currentAvatarImg,
    avatarHash,
    initializeAvatar,
    setAvatar,
  }
}
