import { classificationClass } from '../lib/format'
import type { Classification } from '../types'

export function ClassificationBadge({ value }: { value: Classification }) {
  return <span className={`rounded px-2 py-0.5 text-xs font-semibold ${classificationClass(value)}`}>{value}</span>
}

export function Pill({ children, tone = 'slate' }: { children: string; tone?: 'slate' | 'red' | 'green' | 'amber' }) {
  const tones = {
    slate: 'bg-slate-100 text-slate-700',
    red: 'bg-red-100 text-red-800',
    green: 'bg-green-100 text-green-800',
    amber: 'bg-amber-100 text-amber-800',
  }
  return <span className={`rounded px-2 py-0.5 text-xs font-semibold ${tones[tone]}`}>{children}</span>
}

export function DecisionBadge({ decision }: { decision: 'PASS' | 'WARN' | 'BLOCK' }) {
  const tone = decision === 'BLOCK' ? 'red' : decision === 'WARN' ? 'amber' : 'green'
  return <Pill tone={tone}>{decision}</Pill>
}
