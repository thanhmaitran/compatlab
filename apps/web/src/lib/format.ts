import type { Classification, Severity } from '../types'

export function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`
}

export function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toISOString().replace('T', ' ').slice(0, 16) + ' UTC'
}

export function classificationClass(c: Classification): string {
  switch (c) {
    case 'BREAKING':
      return 'bg-red-100 text-red-800'
    case 'CONDITIONAL':
      return 'bg-amber-100 text-amber-800'
    default:
      return 'bg-green-100 text-green-800'
  }
}

const severityRank: Record<Severity, number> = { INFO: 0, LOW: 1, MEDIUM: 2, HIGH: 3, CRITICAL: 4 }

export function compareSeverity(a: Severity, b: Severity): number {
  return severityRank[b] - severityRank[a]
}

export function isValidEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
}
