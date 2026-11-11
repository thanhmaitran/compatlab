import type { GateView, PolicyView, ReleasePolicy } from '../types'
import { get, post } from './api-client'

export const listPolicies = (projectId: string) => get<PolicyView[]>(`/projects/${projectId}/policies`)
export const createPolicy = (projectId: string, policy: ReleasePolicy) => post<PolicyView>('/policies', { project_id: projectId, policy })
export const evaluateGate = (projectId: string, analysisId: string, policyId: string, contractIds: string[], testRunId: string) =>
  post<GateView>('/gate/evaluate', {
    project_id: projectId,
    analysis_id: analysisId,
    policy_id: policyId,
    contract_ids: contractIds,
    test_run_id: testRunId || undefined,
  })
export const listGates = (projectId: string) => get<GateView[]>(`/projects/${projectId}/gates`)
