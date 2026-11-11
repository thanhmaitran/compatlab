import type { RunView, SuiteView } from '../types'
import { get, post } from './api-client'

export const generateSuite = (versionId: string, analysisId: string, maxCasesPerEndpoint: number) =>
  post<SuiteView>('/test-suites/generate', { version_id: versionId, analysis_id: analysisId || undefined, max_cases_per_endpoint: maxCasesPerEndpoint })
export const runTests = (suiteId: string, versionId: string) => post<RunView>('/test-runs', { suite_id: suiteId, version_id: versionId })
export const listRuns = (projectId: string) => get<RunView[]>(`/projects/${projectId}/test-runs`)
export const getRun = (id: string) => get<RunView>(`/test-runs/${id}`)
