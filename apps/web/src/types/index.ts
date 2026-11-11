export type Classification = 'BREAKING' | 'NON_BREAKING' | 'CONDITIONAL'
export type Direction = 'BACKWARD' | 'FORWARD'
export type Severity = 'INFO' | 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL'
export type Decision = 'PASS' | 'WARN' | 'BLOCK'

export interface Project {
  id: string
  name: string
  description: string
  created_at: string
}

export interface Version {
  id: string
  project_id: string
  label: string
  specification_id: string
  created_at: string
}

export interface UploadResult {
  version: Version
  endpoints: number
  schemas: number
}

export interface Summary {
  breaking: number
  non_breaking: number
  conditional: number
}

export interface Scores {
  backward: number
  forward: number
}

export interface ClassifiedChange {
  id: string
  method?: string
  path?: string
  change_type: string
  location?: string
  description: string
  rule_id: string
  classification: Classification
  severity: Severity
  direction: Direction
  explanation?: string
  remediation?: string
}

export interface AnalysisView {
  id: string
  project_id: string
  base_version_id: string
  target_version_id: string
  created_at: string
  scores?: Scores
  backward: Summary
  forward: Summary
  analysis?: { base_version: string; target_version: string; changes: ClassifiedChange[] }
}

export interface ContractView {
  id: string
  project_id: string
  consumer: string
  provider: string
  interactions: number
  created_at: string
}

export interface InteractionResult {
  description: string
  passed: boolean
  message?: string
}

export interface Verification {
  consumer: string
  provider: string
  interactions: InteractionResult[]
}

export interface VerifyResult {
  passed: boolean
  verification: Verification
}

export interface SuiteView {
  id: string
  project_id: string
  version_id: string
  name: string
  cases: number
  created_at: string
}

export interface TestResult {
  case_id: string
  status: string
  message?: string
}

export interface RunView {
  id: string
  project_id: string
  suite_id: string
  version_id: string
  target: string
  pass_rate: number
  passed: number
  failed: number
  errored: number
  created_at: string
  run?: { results: TestResult[] }
}

export interface ReleasePolicy {
  id?: string
  name: string
  fail_on: Classification[]
  require_contract_tests: boolean
  minimum_pass_rate: number
  include_forward?: boolean
}

export interface PolicyView {
  id: string
  project_id: string
  created_at: string
  policy: ReleasePolicy
}

export interface ConsumerImpact {
  consumer: string
  level: 'AFFECTED' | 'REVIEW'
  rule_id: string
  location?: string
}

export interface GateView {
  id: string
  project_id: string
  analysis_id: string
  policy_id: string
  created_at: string
  report: {
    evaluation?: { decision: Decision; reasons?: string[] }
    consumer_impacts?: ConsumerImpact[]
    contracts?: Verification[]
  }
}

export interface AnalyticsSummary {
  total_analyses: number
  score_trend: { analysis_id: string; base: string; target: string; created_at: string; backward: number; forward: number }[]
  breaking_by_version: { target: string; breaking: number; conditional: number; non_breaking: number }[]
  distribution: Summary
  pass_rate_trend: { run_id: string; created_at: string; pass_rate: number }[]
  contract_trend: { report_id: string; created_at: string; passed: number; total: number }[]
  gate_history: { report_id: string; created_at: string; decision: Decision }[]
  gate_blocks: number
  top_changed_endpoints: { method: string; path: string; count: number }[]
}

export interface SpecParameter {
  name: string
  location: string
  required: boolean
  type: string
}

export interface SpecEndpoint {
  method: string
  path: string
  operation_id?: string
  deprecated?: boolean
  parameters: SpecParameter[]
  responses: string[]
  security: string[]
  request_body?: string
}

export interface SpecSchema {
  name: string
  type: string
  properties: string[]
  required: string[]
}

export interface SpecView {
  version: Version
  title: string
  api_version: string
  openapi_version: string
  endpoints: SpecEndpoint[]
  schemas: SpecSchema[]
  security_schemes: string[]
}
