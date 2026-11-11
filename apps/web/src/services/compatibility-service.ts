import type { AnalysisView, ClassifiedChange } from '../types'
import { get, post } from './api-client'

export const analyze = (projectId: string, baseVersionId: string, targetVersionId: string) =>
  post<AnalysisView>(`/projects/${projectId}/analyze`, { base_version_id: baseVersionId, target_version_id: targetVersionId })
export const listAnalyses = (projectId: string) => get<AnalysisView[]>(`/projects/${projectId}/analyses`)
export const getAnalysis = (id: string) => get<AnalysisView>(`/analyses/${id}`)
export const getChanges = (id: string) => get<ClassifiedChange[]>(`/analyses/${id}/changes`)
