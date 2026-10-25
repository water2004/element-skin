export function landscape(dusk = false) {
  const sky = dusk ? ['#14253b', '#b97479'] : ['#287eb7', '#9bc8c4']
  return `<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="900" viewBox="0 0 1600 900">
    <defs><linearGradient id="sky" x2="0" y2="1"><stop stop-color="${sky[0]}"/><stop offset="1" stop-color="${sky[1]}"/></linearGradient>
    <pattern id="grid" width="100" height="100" patternUnits="userSpaceOnUse"><path d="M100 0H0V100" fill="none" stroke="#fff" stroke-opacity=".1"/></pattern></defs>
    <path fill="url(#sky)" d="M0 0h1600v900H0z"/>
    <circle cx="${dusk ? 400 : 1200}" cy="220" r="90" fill="${dusk ? '#ffe2bd' : '#ffda8b'}"/>
    <path d="M0 590L220 230 490 590 760 290 1090 650 1380 260 1600 590V900H0Z" fill="${dusk ? '#34445e' : '#41768e'}"/>
    <path d="M0 700L340 490 620 740 920 430 1280 720 1600 490V900H0Z" fill="${dusk ? '#192f47' : '#275e6c'}"/>
    <path d="M0 770Q400 640 800 790T1600 770V900H0Z" fill="${dusk ? '#122d38' : '#194e4e'}"/>
    <path fill="url(#grid)" d="M0 0h1600v900H0z"/>
    <g fill="#fff" opacity=".7" font-family="sans-serif" font-size="22">
      <text x="40" y="60">LEFT · ${dusk ? 'DUSK' : 'DAY'}</text><text x="1320" y="60">RIGHT · 1600 × 900</text>
    </g>
  </svg>`
}

export function cubeFace(face) {
  const colors = ['#3d879d', '#508688', '#7b7797', '#a77e76', '#3d618b', '#28594a']
  return `<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512">
    <defs><linearGradient id="sky" x2="0" y2="1"><stop stop-color="${colors[face]}"/><stop offset="1" stop-color="#c1cbb7"/></linearGradient></defs>
    <path fill="url(#sky)" d="M0 0h512v512H0z"/>
    <path fill="#476358" d="M0 310h60v-90h70v50h100v-120h90v140h80v-70h112v292H0z"/>
    <path fill="#214a43" d="M0 390h110v-50h110v60h100v-85h90v65h102v132H0z"/>
    <g fill="none" stroke="#fff" stroke-opacity=".15"><path d="M0 256h512M256 0v512"/><circle cx="256" cy="256" r="100"/></g>
    <text x="25" y="55" fill="#fff" font-size="24" font-family="sans-serif">PANORAMA ${face}</text>
  </svg>`
}

const base = {
  enabled: true,
  duration_ms: 6000,
  overlay_opacity_light: 0.15,
  overlay_opacity_dark: 0.5,
  start_yaw: 0,
  start_pitch: 0,
  yaw_speed_dps: 5,
  pitch_speed_dps: 0,
  created_at: 0,
  updated_at: 0,
}

export function mediaFor(kind) {
  const image = {
    ...base,
    id: 'local-day',
    title: '日间山景',
    type: 'image',
    storage_path: 'local-day.svg',
    sort_order: 0,
  }
  const dusk = {
    ...base,
    id: 'local-dusk',
    title: '暮色山景',
    type: 'image',
    storage_path: 'local-dusk.svg',
    sort_order: 1,
  }
  const panorama = {
    ...base,
    id: 'local-panorama',
    title: '全景网格',
    type: 'panorama',
    storage_path: 'local-panorama',
    sort_order: 2,
  }
  return kind === 'panorama' ? [panorama] : kind === 'carousel' ? [image, dusk, panorama] : [image]
}

export const settings = {
  site_name: 'Element Skin · 本地验证',
  site_subtitle: '连续调整窗口大小，观察背景、玻璃按钮与轮播',
  footer_text: 'Issue #19 · 合成素材 · 仅供本机验证',
  enable_skin_library: false,
  allow_register: true,
  easter_eggs: { enabled: false, items: [] },
}

export const user = {
  id: 'local-verification-user',
  email: 'local@example.test',
  display_name: '本地验证账号',
  permissions: ['account.read.self', 'account.update.self'],
  avatar_hash: null,
  profile_count: 0,
  texture_count: 0,
}
