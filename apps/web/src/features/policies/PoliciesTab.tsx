import { useState } from 'react'
import type { FormEvent } from 'react'
import { ClassificationBadge, DecisionBadge } from '../../components/Badge'
import { Async, EmptyState } from '../../components/States'
import { useAsync } from '../../hooks/useAsync'
import { formatDate } from '../../lib/format'
import { listAnalyses } from '../../services/compatibility-service'
import { listContracts } from '../../services/contract-service'
import { createPolicy, evaluateGate, listGates, listPolicies } from '../../services/policy-service'
import { listVersions } from '../../services/project-service'
import { listRuns } from '../../services/testing-service'
import type { Classification, GateView } from '../../types'

const classifications: Classification[] = ['BREAKING', 'CONDITIONAL', 'NON_BREAKING']

export function PoliciesTab({ projectId }: { projectId: string }) {
  const policies = useAsync(() => listPolicies(projectId), [projectId])
  const gates = useAsync(() => listGates(projectId), [projectId])
  const analyses = useAsync(() => listAnalyses(projectId), [projectId])
  const versions = useAsync(() => listVersions(projectId), [projectId])
  const contracts = useAsync(() => listContracts(projectId), [projectId])
  const runs = useAsync(() => listRuns(projectId), [projectId])

  const [name, setName] = useState('')
  const [failOn, setFailOn] = useState<Classification[]>([])
  const [requireContracts, setRequireContracts] = useState(false)
  const [minRate, setMinRate] = useState(0)
  const [policyError, setPolicyError] = useState('')

  const [analysisId, setAnalysisId] = useState('')
  const [policyId, setPolicyId] = useState('')
  const [contractIds, setContractIds] = useState<string[]>([])
  const [runId, setRunId] = useState('')
  const [gateError, setGateError] = useState('')
  const [result, setResult] = useState<GateView | null>(null)
  const labelOf = (id: string) => (versions.data ?? []).find((v) => v.id === id)?.label ?? id

  function toggle<T>(list: T[], value: T): T[] {
    return list.includes(value) ? list.filter((x) => x !== value) : [...list, value]
  }

  async function submitPolicy(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setPolicyError('')
    if (name.trim() === '') return setPolicyError('Enter a policy name.')
    if (minRate < 0 || minRate > 100) return setPolicyError('The minimum pass rate must be between 0 and 100.')
    if (!window.confirm('Create this release policy?')) return
    try {
      await createPolicy(projectId, { name: name.trim(), fail_on: failOn, require_contract_tests: requireContracts, minimum_pass_rate: minRate })
      setName('')
      policies.reload()
    } catch (err) {
      setPolicyError(err instanceof Error ? err.message : 'Could not create the policy')
    }
  }

  async function submitGate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setGateError('')
    setResult(null)
    if (!analysisId || !policyId) return setGateError('Choose an analysis and a policy.')
    try {
      const view = await evaluateGate(projectId, analysisId, policyId, contractIds, runId)
      setResult(view)
      gates.reload()
    } catch (err) {
      setGateError(err instanceof Error ? err.message : 'Evaluation failed')
    }
  }

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-semibold">Release policies</h2>
      <form onSubmit={submitPolicy} className="card space-y-3" noValidate>
        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <label htmlFor="policy-name" className="label">Policy name</label>
            <input id="policy-name" className="input" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <label htmlFor="min-rate" className="label">Minimum test pass rate (%)</label>
            <input id="min-rate" type="number" className="input" value={minRate} onChange={(e) => setMinRate(Number(e.target.value))} />
          </div>
        </div>
        <fieldset>
          <legend className="label">Block the release on</legend>
          <div className="flex gap-4">
            {classifications.map((c) => (
              <label key={c} className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={failOn.includes(c)} onChange={() => setFailOn(toggle(failOn, c))} />
                {c}
              </label>
            ))}
          </div>
        </fieldset>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={requireContracts} onChange={(e) => setRequireContracts(e.target.checked)} />
          Require contract verification
        </label>
        {policyError && <p role="alert" className="text-sm text-red-700">{policyError}</p>}
        <button type="submit" className="btn">Create policy</button>
      </form>
      <Async state={policies} isEmpty={(l) => l.length === 0} empty={<EmptyState title="No policies yet" hint="Only organization admins can create policies." />}>
        {(list) => (
          <ul className="space-y-2">
            {list.map((p) => (
              <li key={p.id} className="card text-sm">
                <span className="font-medium">{p.policy.name}</span> — blocks on {p.policy.fail_on.length > 0 ? p.policy.fail_on.map((c) => <ClassificationBadge key={c} value={c} />) : 'nothing'}
                {p.policy.require_contract_tests && ' · contracts required'}
                {p.policy.minimum_pass_rate > 0 && ` · pass rate ≥ ${p.policy.minimum_pass_rate}%`}
              </li>
            ))}
          </ul>
        )}
      </Async>

      <h2 className="text-lg font-semibold">Release gate</h2>
      <form onSubmit={submitGate} className="card space-y-3" noValidate>
        <div className="grid gap-4 md:grid-cols-3">
          <div>
            <label htmlFor="gate-analysis" className="label">Analysis</label>
            <select id="gate-analysis" className="input" value={analysisId} onChange={(e) => setAnalysisId(e.target.value)}>
              <option value="">Select…</option>
              {(analyses.data ?? []).map((a) => <option key={a.id} value={a.id}>{labelOf(a.base_version_id)} → {labelOf(a.target_version_id)}</option>)}
            </select>
          </div>
          <div>
            <label htmlFor="gate-policy" className="label">Policy</label>
            <select id="gate-policy" className="input" value={policyId} onChange={(e) => setPolicyId(e.target.value)}>
              <option value="">Select…</option>
              {(policies.data ?? []).map((p) => <option key={p.id} value={p.id}>{p.policy.name}</option>)}
            </select>
          </div>
          <div>
            <label htmlFor="gate-run" className="label">Test run (optional)</label>
            <select id="gate-run" className="input" value={runId} onChange={(e) => setRunId(e.target.value)}>
              <option value="">None</option>
              {(runs.data ?? []).map((r) => <option key={r.id} value={r.id}>{formatDate(r.created_at)}</option>)}
            </select>
          </div>
        </div>
        {(contracts.data ?? []).length > 0 && (
          <fieldset>
            <legend className="label">Verify consumer contracts</legend>
            <div className="flex flex-wrap gap-4">
              {(contracts.data ?? []).map((c) => (
                <label key={c.id} className="flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={contractIds.includes(c.id)} onChange={() => setContractIds(toggle(contractIds, c.id))} />
                  Contract: {c.consumer}
                </label>
              ))}
            </div>
          </fieldset>
        )}
        {gateError && <p role="alert" className="text-sm text-red-700">{gateError}</p>}
        <button type="submit" className="btn">Evaluate release</button>
      </form>
      {result?.report.evaluation && (
        <div className="card space-y-2" role="status">
          <p className="text-lg font-semibold">Decision: <DecisionBadge decision={result.report.evaluation.decision} /></p>
          <ul className="list-disc pl-5 text-sm">
            {(result.report.evaluation.reasons ?? []).map((r) => <li key={r}>{r}</li>)}
          </ul>
          {(result.report.consumer_impacts ?? []).length > 0 && (
            <div>
              <h3 className="text-sm font-semibold">Consumer impact</h3>
              <ul className="text-sm">
                {(result.report.consumer_impacts ?? []).map((i, n) => <li key={n}>{i.consumer}: {i.level} ({i.rule_id}{i.location ? `, ${i.location}` : ''})</li>)}
              </ul>
            </div>
          )}
        </div>
      )}
      <Async state={gates} isEmpty={(l) => l.length === 0} empty={<EmptyState title="No gate evaluations yet" />}>
        {(list) => (
          <table className="card w-full text-left text-sm">
            <thead className="text-slate-500"><tr><th>Date</th><th>Decision</th><th>Reasons</th></tr></thead>
            <tbody>
              {list.map((g) => (
                <tr key={g.id} className="border-t border-slate-100">
                  <td className="py-2">{formatDate(g.created_at)}</td>
                  <td>{g.report.evaluation ? <DecisionBadge decision={g.report.evaluation.decision} /> : '–'}</td>
                  <td>{(g.report.evaluation?.reasons ?? []).join('; ') || '–'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Async>
    </div>
  )
}
