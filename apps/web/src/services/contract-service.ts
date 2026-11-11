import type { ContractView, VerifyResult } from '../types'
import { get, post } from './api-client'

export const listContracts = (projectId: string) => get<ContractView[]>(`/projects/${projectId}/contracts`)
export const registerContract = (projectId: string, contract: unknown) => post<ContractView>('/contracts', { project_id: projectId, contract })
export const verifyContract = (contractId: string, versionId: string) => post<VerifyResult>(`/contracts/${contractId}/verify`, { version_id: versionId })
