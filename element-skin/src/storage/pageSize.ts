function parsePageSize(value: unknown): number | null {
  if (typeof value !== 'number' && typeof value !== 'string') return null
  if (typeof value === 'string' && !value.trim()) return null
  const parsed = Number(value)
  return Number.isFinite(parsed) ? Math.min(100, Math.max(1, Math.round(parsed))) : null
}

export function normalizePageSize(value: unknown, fallback = 20): number {
  return parsePageSize(value) ?? parsePageSize(fallback) ?? 20
}
