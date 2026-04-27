import { ref, watch } from 'vue';
import * as skinview3d from 'skinview3d';
import axios from 'axios';

// Global reactive state to ensure sync across components
const currentAvatarImg = ref(null);
const avatarHash = ref(null);

// Sequential generation queue — ensures only ONE WebGL context exists at a time
let _generationQueue = Promise.resolve();

/**
 * Generate a 3D headshot from a skin hash, using a single WebGL instance.
 * This is the low-level generator; callers should use getAvatarForHash() instead.
 */
function _doGenerateAvatar(hash, model = 'default') {
  return new Promise((resolve) => {
    const canvas = document.createElement('canvas');
    const viewer = new skinview3d.SkinViewer({
      canvas,
      width: 256,
      height: 256,
      model: model,
      preserveDrawingBuffer: true
    });

    // Hide body parts
    if (viewer.playerObject.skin.body) viewer.playerObject.skin.body.visible = false;
    if (viewer.playerObject.skin.leftArm) viewer.playerObject.skin.leftArm.visible = false;
    if (viewer.playerObject.skin.rightArm) viewer.playerObject.skin.rightArm.visible = false;
    if (viewer.playerObject.skin.leftLeg) viewer.playerObject.skin.leftLeg.visible = false;
    if (viewer.playerObject.skin.rightLeg) viewer.playerObject.skin.rightLeg.visible = false;
    if (viewer.playerObject.cape) viewer.playerObject.cape.visible = false;
    if (viewer.playerObject.elytra) viewer.playerObject.elytra.visible = false;

    // Adjust camera to perfectly frame the head
    viewer.playerWrapper.position.y = -12; 
    viewer.playerWrapper.rotation.y = 0; 
    viewer.playerWrapper.rotation.x = 0;
    viewer.zoom = 4.0;
    viewer.autoRotate = false;

    const base = import.meta.env.BASE_URL;
    const textureUrl = `${base}static/textures/${hash}.png`.replace(/\/+/g, '/');
    viewer.loadSkin(textureUrl).then(() => {
      viewer.render();
      const base64 = canvas.toDataURL();
      localStorage.setItem(`avatar_cache_${hash}`, base64);
      viewer.dispose();
      resolve(base64);
    }).catch((e) => {
      console.error('Failed to generate avatar for hash', hash, e);
      viewer.dispose();
      resolve(null);
    });
  });
}

/**
 * Get avatar image for any texture hash.
 * Returns cached base64 from localStorage instantly, or generates sequentially.
 * Safe to call many times — WebGL instances are serialized via a promise queue.
 *
 * @param {string} hash - Skin texture hash
 * @param {string} [model='default'] - 'default' or 'slim'
 * @returns {Promise<string|null>} base64 image or null
 */
export function getAvatarForHash(hash, model = 'default') {
  if (!hash) return Promise.resolve(null);

  const cached = localStorage.getItem(`avatar_cache_${hash}`);
  if (cached) return Promise.resolve(cached);

  // Enqueue generation to avoid concurrent WebGL contexts
  const task = _generationQueue.then(() => _doGenerateAvatar(hash, model));
  _generationQueue = task.catch(() => {});  // swallow errors to keep queue alive
  return task;
}

export function useAvatar() {
  const texturesUrl = (hash) => {
    if (!hash) return '';
    const base = import.meta.env.BASE_URL;
    return `${base}static/textures/${hash}.png`.replace(/\/+/g, '/');
  };

  /**
   * Generate 3D headshot and cache it (delegates to shared queue)
   */
  async function generateAndCacheAvatar(hash, model = 'default') {
    return getAvatarForHash(hash, model);
  }

  /**
   * Sync avatar state from a hash (from backend)
   */
  async function initializeAvatar(hash, model = 'default') {
    if (!hash) {
      currentAvatarImg.value = null;
      avatarHash.value = null;
      return;
    }

    avatarHash.value = hash;
    const cached = localStorage.getItem(`avatar_cache_${hash}`);
    if (cached) {
      currentAvatarImg.value = cached;
    } else {
      const generated = await generateAndCacheAvatar(hash, model);
      currentAvatarImg.value = generated;
    }
  }

  /**
   * Set new avatar and sync to backend
   */
  async function setAvatar(hash, model = 'default') {
    try {
      // Sync to backend first
      await axios.patch('/me', { avatar_hash: hash }, {
        headers: { Authorization: `Bearer ${localStorage.getItem('jwt')}` }
      });
      
      // Update local state and cache
      await initializeAvatar(hash, model);
      
      // Legacy event for non-composable components if any
      window.dispatchEvent(new CustomEvent('avatar-changed', { detail: hash }));
      return true;
    } catch (e) {
      console.error('Failed to set avatar:', e);
      throw e;
    }
  }

  return {
    currentAvatarImg,
    avatarHash,
    initializeAvatar,
    setAvatar
  };
}

