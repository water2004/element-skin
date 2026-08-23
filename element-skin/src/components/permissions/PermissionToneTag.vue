<template>
  <button v-if="clickable" type="button" :title="title" :class="rootClasses" @click="emit('click')">
    <span>{{ label }}</span>
    <span v-if="count !== undefined" :class="countClasses">{{ count }}</span>
  </button>
  <span v-else :title="title" :class="rootClasses">
    <span v-if="badgeLabel" :class="badgeClasses">{{ badgeLabel }}</span>
    <span>{{ label }}</span>
    <button
      v-if="removable"
      type="button"
      class="ml-0.5 inline-flex h-4 w-4 items-center justify-center rounded-full opacity-70 transition hover:bg-black/10 hover:opacity-100 dark:hover:bg-white/10"
      @click="emit('remove')"
    >
      <el-icon :size="12"><Close /></el-icon>
    </button>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Close } from '@element-plus/icons-vue'

type PermissionTone = 'emerald' | 'sky' | 'violet' | 'amber' | 'rose' | 'slate' | 'cyan'
type PermissionTagVariant = 'category' | 'permission'
type PermissionBadgeTone = 'allow' | 'deny'

interface PermissionToneClasses {
  chip: string
  activeChip: string
  count: string
  activeCount: string
  pill: string
}

const props = withDefaults(
  defineProps<{
    label: string
    tone: PermissionTone
    variant?: PermissionTagVariant
    active?: boolean
    clickable?: boolean
    count?: number
    title?: string
    removable?: boolean
    badgeLabel?: string
    badgeTone?: PermissionBadgeTone
  }>(),
  {
    variant: 'permission',
    active: false,
    clickable: false,
    removable: false,
  },
)

const emit = defineEmits<{
  click: []
  remove: []
}>()

const permissionToneClassMap: Record<PermissionTone, PermissionToneClasses> = {
  emerald: {
    chip: 'bg-emerald-50 text-emerald-700 ring-emerald-200 hover:bg-emerald-100 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/30 dark:hover:bg-emerald-500/20',
    activeChip:
      'bg-emerald-600 text-white ring-emerald-500 shadow-sm shadow-emerald-500/20 dark:bg-emerald-500 dark:text-emerald-950 dark:ring-emerald-400',
    count: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200',
    activeCount: 'bg-white/20 text-white dark:bg-emerald-950/20 dark:text-emerald-950',
    pill: 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/30',
  },
  sky: {
    chip: 'bg-sky-50 text-sky-700 ring-sky-200 hover:bg-sky-100 dark:bg-sky-500/10 dark:text-sky-300 dark:ring-sky-500/30 dark:hover:bg-sky-500/20',
    activeChip:
      'bg-sky-600 text-white ring-sky-500 shadow-sm shadow-sky-500/20 dark:bg-sky-500 dark:text-sky-950 dark:ring-sky-400',
    count: 'bg-sky-100 text-sky-700 dark:bg-sky-500/20 dark:text-sky-200',
    activeCount: 'bg-white/20 text-white dark:bg-sky-950/20 dark:text-sky-950',
    pill: 'bg-sky-50 text-sky-700 ring-sky-200 dark:bg-sky-500/10 dark:text-sky-300 dark:ring-sky-500/30',
  },
  violet: {
    chip: 'bg-violet-50 text-violet-700 ring-violet-200 hover:bg-violet-100 dark:bg-violet-500/10 dark:text-violet-300 dark:ring-violet-500/30 dark:hover:bg-violet-500/20',
    activeChip:
      'bg-violet-600 text-white ring-violet-500 shadow-sm shadow-violet-500/20 dark:bg-violet-500 dark:text-violet-950 dark:ring-violet-400',
    count: 'bg-violet-100 text-violet-700 dark:bg-violet-500/20 dark:text-violet-200',
    activeCount: 'bg-white/20 text-white dark:bg-violet-950/20 dark:text-violet-950',
    pill: 'bg-violet-50 text-violet-700 ring-violet-200 dark:bg-violet-500/10 dark:text-violet-300 dark:ring-violet-500/30',
  },
  amber: {
    chip: 'bg-amber-50 text-amber-700 ring-amber-200 hover:bg-amber-100 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/30 dark:hover:bg-amber-500/20',
    activeChip: 'bg-amber-500 text-amber-950 ring-amber-400 shadow-sm shadow-amber-500/20',
    count: 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-200',
    activeCount: 'bg-amber-950/10 text-amber-950',
    pill: 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/30',
  },
  rose: {
    chip: 'bg-rose-50 text-rose-700 ring-rose-200 hover:bg-rose-100 dark:bg-rose-500/10 dark:text-rose-300 dark:ring-rose-500/30 dark:hover:bg-rose-500/20',
    activeChip:
      'bg-rose-600 text-white ring-rose-500 shadow-sm shadow-rose-500/20 dark:bg-rose-500 dark:text-rose-950 dark:ring-rose-400',
    count: 'bg-rose-100 text-rose-700 dark:bg-rose-500/20 dark:text-rose-200',
    activeCount: 'bg-white/20 text-white dark:bg-rose-950/20 dark:text-rose-950',
    pill: 'bg-rose-50 text-rose-700 ring-rose-200 dark:bg-rose-500/10 dark:text-rose-300 dark:ring-rose-500/30',
  },
  slate: {
    chip: 'bg-slate-50 text-slate-700 ring-slate-200 hover:bg-slate-100 dark:bg-slate-500/10 dark:text-slate-300 dark:ring-slate-500/30 dark:hover:bg-slate-500/20',
    activeChip:
      'bg-slate-700 text-white ring-slate-600 shadow-sm shadow-slate-500/20 dark:bg-slate-300 dark:text-slate-950 dark:ring-slate-200',
    count: 'bg-slate-100 text-slate-700 dark:bg-slate-500/20 dark:text-slate-200',
    activeCount: 'bg-white/20 text-white dark:bg-slate-950/20 dark:text-slate-950',
    pill: 'bg-slate-50 text-slate-700 ring-slate-200 dark:bg-slate-500/10 dark:text-slate-300 dark:ring-slate-500/30',
  },
  cyan: {
    chip: 'bg-cyan-50 text-cyan-700 ring-cyan-200 hover:bg-cyan-100 dark:bg-cyan-500/10 dark:text-cyan-300 dark:ring-cyan-500/30 dark:hover:bg-cyan-500/20',
    activeChip:
      'bg-cyan-600 text-white ring-cyan-500 shadow-sm shadow-cyan-500/20 dark:bg-cyan-500 dark:text-cyan-950 dark:ring-cyan-400',
    count: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-500/20 dark:text-cyan-200',
    activeCount: 'bg-white/20 text-white dark:bg-cyan-950/20 dark:text-cyan-950',
    pill: 'bg-cyan-50 text-cyan-700 ring-cyan-200 dark:bg-cyan-500/10 dark:text-cyan-300 dark:ring-cyan-500/30',
  },
}

const toneClasses = computed(() => permissionToneClassMap[props.tone])

const rootClasses = computed(() => {
  if (props.variant === 'category') {
    return [
      'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold ring-1 transition',
      'hover:-translate-y-px focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--color-background)]',
      props.active ? toneClasses.value.activeChip : toneClasses.value.chip,
    ]
  }
  return [
    'inline-flex max-w-full items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium ring-1 transition hover:-translate-y-px',
    toneClasses.value.pill,
  ]
})

const countClasses = computed(() => [
  'rounded-full px-1.5 py-0.5 text-[10px] leading-none',
  props.active ? toneClasses.value.activeCount : toneClasses.value.count,
])

const badgeClasses = computed(() => [
  'rounded px-1 py-0.5 text-[10px] font-semibold leading-none',
  props.badgeTone === 'deny'
    ? 'bg-rose-100 text-rose-700 dark:bg-rose-500/20 dark:text-rose-200'
    : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200',
])
</script>
