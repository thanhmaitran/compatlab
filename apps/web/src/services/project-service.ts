import type { Project, UploadResult, Version } from '../types'
import { get, post } from './api-client'

export const listProjects = () => get<Project[]>('/projects')
export const getProject = (id: string) => get<Project>(`/projects/${id}`)
export const createProject = (name: string, description: string) => post<Project>('/projects', { name, description })
export const listVersions = (projectId: string) => get<Version[]>(`/projects/${projectId}/versions`)
export const uploadSpecification = (projectId: string, versionLabel: string, filename: string, content: string) =>
  post<UploadResult>(`/projects/${projectId}/specifications`, { version_label: versionLabel, filename, content })
export const inspectVersion = (versionId: string) => get<import('../types').SpecView>(`/versions/${versionId}/specification`)
