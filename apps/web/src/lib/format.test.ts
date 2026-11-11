import { describe, expect, it } from 'vitest'
import { classificationClass, compareSeverity, formatDate, formatPercent, isValidEmail } from './format'

describe('format helpers', () => {
  it('formats percentages with one decimal', () => {
    expect(formatPercent(95)).toBe('95.0%')
    expect(formatPercent(66.666)).toBe('66.7%')
  })

  it('formats dates in UTC and tolerates invalid input', () => {
    expect(formatDate('2026-01-02T03:04:05Z')).toBe('2026-01-02 03:04 UTC')
    expect(formatDate('not a date')).toBe('not a date')
  })

  it('maps classifications to distinct styles', () => {
    expect(classificationClass('BREAKING')).toContain('red')
    expect(classificationClass('CONDITIONAL')).toContain('amber')
    expect(classificationClass('NON_BREAKING')).toContain('green')
  })

  it('sorts severities from most to least severe', () => {
    expect(['LOW', 'CRITICAL', 'MEDIUM'].sort((a, b) => compareSeverity(a as never, b as never))).toEqual(['CRITICAL', 'MEDIUM', 'LOW'])
  })

  it('validates e-mail addresses', () => {
    expect(isValidEmail('a@b.co')).toBe(true)
    expect(isValidEmail('nope')).toBe(false)
    expect(isValidEmail('a b@c.de')).toBe(false)
  })
})
