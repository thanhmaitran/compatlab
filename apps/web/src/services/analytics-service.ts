import type { AnalyticsSummary } from '../types'
import { get } from './api-client'

export const getAnalytics = (projectId: string) => get<AnalyticsSummary>(`/projects/${projectId}/analytics`)
