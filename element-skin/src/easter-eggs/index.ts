import type { Router } from 'vue-router'

export type EasterEggCleanup = () => void

export interface EasterEggModule {
  start: () => void | EasterEggCleanup | Promise<void | EasterEggCleanup>
}

interface EasterEggDefinition {
  id: string
  name: string
  description: string
  htmlClass?: string
  active: (date: Date) => boolean
  load: () => Promise<EasterEggModule>
}

export interface EasterEggConfig {
  enabled?: string[]
}

const EASTER_EGG_DISABLED_KEY = 'disableEasterEgg'
const LEGACY_EASTER_EGG_DISABLED_KEY = 'disableMeowEasterEgg'

function isChineseCalendarDay(date: Date, month: number, day: number): boolean {
  try {
    const parts = new Intl.DateTimeFormat('zh-CN-u-ca-chinese', {
      month: 'numeric',
      day: 'numeric',
    }).formatToParts(date)
    const lunarMonth = Number(parts.find((part) => part.type === 'month')?.value)
    const lunarDay = Number(parts.find((part) => part.type === 'day')?.value)
    return lunarMonth === month && lunarDay === day
  } catch {
    return false
  }
}

const definitions: EasterEggDefinition[] = [
  {
    id: 'spring-festival',
    name: '春节',
    description: '农历正月初一启用春节主题配色、点击火花与背景烟花。',
    htmlClass: 'easter-egg-spring-festival',
    active: (date) => isChineseCalendarDay(date, 1, 1),
    load: () => import('./springFestival'),
  },
  {
    id: 'april-fools',
    name: '愚人节',
    description: '4 月 1 日启用点击元素物理效果。',
    htmlClass: 'easter-egg-april-fools',
    active: (date) => date.getMonth() === 3 && date.getDate() === 1,
    load: () => import('./aprilFools'),
  },
  {
    id: 'qingming',
    name: '清明',
    description: '4 月 4 日至 4 月 5 日启用雨纷纷的细雨效果。',
    htmlClass: 'easter-egg-qingming',
    active: (date) => date.getMonth() === 3 && date.getDate() >= 4 && date.getDate() <= 5,
    load: () => import('./qingming'),
  },
  {
    id: 'children-day',
    name: '儿童节',
    description: '6 月 1 日启用轻量彩色气泡效果。',
    htmlClass: 'easter-egg-children-day',
    active: (date) => date.getMonth() === 5 && date.getDate() === 1,
    load: () => import('./childrenDay'),
  },
  {
    id: 'dragon-boat',
    name: '端午节',
    description: '农历五月初五启用粽子重力、碰撞和倾斜响应效果。',
    htmlClass: 'easter-egg-dragon-boat',
    active: (date) => isChineseCalendarDay(date, 5, 5),
    load: () => import('./dragonBoat'),
  },
  {
    id: 'christmas',
    name: '圣诞节',
    description: '12 月 24 日至 12 月 25 日启用飘雪效果。',
    htmlClass: 'easter-egg-christmas',
    active: (date) => date.getMonth() === 11 && date.getDate() >= 24 && date.getDate() <= 25,
    load: () => import('./christmas'),
  },
]

let activeCleanup: EasterEggCleanup | null = null
let activeClass: string | null = null
let runToken = 0
let serverConfig: EasterEggConfig | null = null

function hasDOM(): boolean {
  return typeof window !== 'undefined' && typeof document !== 'undefined'
}

function stopActive(): void {
  if (activeCleanup) {
    activeCleanup()
    activeCleanup = null
  }
  if (activeClass) {
    document.documentElement.classList.remove(activeClass)
    activeClass = null
  }
}

export function isEasterEggDisabled(): boolean {
  if (!hasDOM()) return true
  return localStorage.getItem(EASTER_EGG_DISABLED_KEY) === '1' || localStorage.getItem(LEGACY_EASTER_EGG_DISABLED_KEY) === '1'
}

export function setEasterEggDisabled(disabled: boolean): void {
  if (!hasDOM()) return
  if (disabled) {
    localStorage.setItem(EASTER_EGG_DISABLED_KEY, '1')
    localStorage.removeItem(LEGACY_EASTER_EGG_DISABLED_KEY)
    cleanupEasterEgg()
    return
  }
  localStorage.removeItem(EASTER_EGG_DISABLED_KEY)
  localStorage.removeItem(LEGACY_EASTER_EGG_DISABLED_KEY)
  void refreshEasterEgg()
}

export function availableEasterEggs(): Array<Pick<EasterEggDefinition, 'id' | 'name' | 'description'>> {
  return definitions.map(({ id, name, description }) => ({ id, name, description }))
}

export function activeEasterEggFor(date = new Date()): EasterEggDefinition | null {
  const enabled = serverConfig?.enabled
  return definitions.find((definition) => {
    if (enabled && !enabled.includes(definition.id)) return false
    return definition.active(date)
  }) || null
}

export function setServerEasterEggConfig(config?: EasterEggConfig | null): void {
  serverConfig = config || null
  void refreshEasterEgg()
}

function resolveEasterEgg(date: Date): EasterEggDefinition | null {
  return activeEasterEggFor(date)
}

async function startDefinition(definition: EasterEggDefinition, token: number): Promise<void> {
  const mod = await definition.load()
  if (token !== runToken) return

  if (definition.htmlClass) {
    document.documentElement.classList.add(definition.htmlClass)
    activeClass = definition.htmlClass
  }

  const cleanup = await mod.start()
  if (token !== runToken) {
    if (cleanup) cleanup()
    return
  }
  activeCleanup = cleanup || null
}

export function cleanupEasterEgg(): void {
  runToken += 1
  if (!hasDOM()) return
  stopActive()
}

export async function refreshEasterEgg(date = new Date()): Promise<void> {
  if (!hasDOM()) return

  const token = runToken + 1
  runToken = token
  stopActive()

  if (isEasterEggDisabled()) return

  const definition = resolveEasterEgg(date)
  if (!definition) return

  try {
    await startDefinition(definition, token)
  } catch (error) {
    console.warn(`Failed to start easter egg "${definition.id}":`, error)
    if (token === runToken) stopActive()
  }
}

export async function startEasterEggForDebug(id: string): Promise<boolean> {
  if (!hasDOM()) return false

  const definition = definitions.find((item) => item.id === id)
  if (!definition) return false

  const token = runToken + 1
  runToken = token
  stopActive()

  try {
    await startDefinition(definition, token)
    return token === runToken
  } catch (error) {
    console.warn(`Failed to start easter egg "${definition.id}":`, error)
    if (token === runToken) stopActive()
    return false
  }
}

export function installEasterEggRouterHooks(router: Router): void {
  router.beforeEach(() => {
    cleanupEasterEgg()
  })
  router.afterEach(() => {
    void refreshEasterEgg()
  })
}

export function installEasterEggDevTools(): void {
  if (!hasDOM() || !import.meta.env.DEV) return

  window.elementSkinEasterEggs = {
    list: availableEasterEggs,
    start: startEasterEggForDebug,
    stop: cleanupEasterEgg,
    refreshAt: (date: string | Date) => refreshEasterEgg(new Date(date)),
    setDisabled: setEasterEggDisabled,
  }
}
