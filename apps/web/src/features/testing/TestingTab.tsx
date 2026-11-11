import { useState } from 'react'
import type { FormEvent } from 'react'
import { Async, EmptyState } from '../../components/States'
import { useAsync } from '../../hooks/useAsync'
import { formatDate, formatPercent } from '../../lib/format'
import { listAnalyses } from '../../services/compatibility-service'
import { listVersions } from '../../services/project-service'
import { generateSuite, getRun, listRuns, runTests } from '../../services/testing-service'
import type { RunView } from '../../types'

export function TestingTab({ projectId }: { projectId: string }) {
  const runs = useAsync(() => listRuns(projectId), [projectId])
  const versions = useAsync(() => listVersions(projectId), [projectId])
  const analyses = useAsync(() => listAnalyses(projectId), [projectId])
  const [versionId, setVersionId] = useState('')
  const [analysisId, setAnalysisId] = useState('')
  const [maxCases, setMaxCases] = useState(20)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [detail, setDetail] = useState<RunView | null>(null)
  const labelOf = (id: string) => (versions.data ?? []).find((v) => v.id === id)?.label ?? id

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')
    if (!versionId) return setError('Choose the version to test.')
    if (!Number.isInteger(maxCases) || maxCases < 1 || maxCases > 500) return setError('Cases per endpoint must be between 1 and 500.')
    if (!window.confirm('Generate tests and run them against a mock of this version?')) return
    setBusy(true)
    try {
      const suite = await generateSuite(versionId, analysisId, maxCases)
      await runTests(suite.id, versionId)
      runs.reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Test run failed')
    } finally {
      setBusy(false)
    }
  }

  async function showDetails(id: string) {
    setDetail(await getRun(id))
  }

  return (
    <div className="space-y-6">
      <form onSubmit={submit} className="card grid gap-4 md:grid-cols-4" noValidate>
        <div>
          <label htmlFor="test-version" className="label">Version to test</label>
          <select id="test-version" className="input" value={versionId} onChange={(e) => setVersionId(e.target.value)}>
            <option value="">Select…</option>
            {(versions.data ?? []).map((v) => <option key={v.id} value={v.id}>{v.label}</option>)}
          </select>
        </div>
        <div>
          <label htmlFor="test-analysis" className="label">Prioritize by analysis (optional)</label>
          <select id="test-analysis" className="input" value={analysisId} onChange={(e) => setAnalysisId(e.target.value)}>
            <option value="">None</option>
            {(analyses.data ?? []).map((a) => <option key={a.id} value={a.id}>{labelOf(a.base_version_id)} → {labelOf(a.target_version_id)}</option>)}
          </select>
        </div>
        <div>
          <label htmlFor="max-cases" className="label">Cases per endpoint</label>
          <input id="max-cases" type="number" className="input" value={maxCases} onChange={(e) => setMaxCases(Number(e.target.value))} />
        </div>
        <div className="flex items-end"><button type="submit" className="btn" disabled={busy}>Generate and run tests</button></div>
        {error && <p role="alert" className="text-sm text-red-700 md:col-span-4">{error}</p>}
      </form>
      <Async state={runs} isEmpty={(l) => l.length === 0} empty={<EmptyState title="No test runs yet" hint="Generated tests run against an in-process mock, never against real systems." />}>
        {(list) => (
          <table className="card w-full text-left text-sm">
            <thead className="text-slate-500"><tr><th>Date</th><th>Version</th><th>Pass rate</th><th>Passed</th><th>Failed</th><th>Errored</th><th /></tr></thead>
            <tbody>
              {list.map((r) => (
                <tr key={r.id} className="border-t border-slate-100">
                  <td className="py-2">{formatDate(r.created_at)}</td>
                  <td>{labelOf(r.version_id)}</td>
                  <td>{formatPercent(r.pass_rate)}</td>
                  <td>{r.passed}</td><td>{r.failed}</td><td>{r.errored}</td>
                  <td><button type="button" className="text-indigo-700" onClick={() => showDetails(r.id)}>Details</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Async>
      {detail && (
        <div className="card" role="status">
          <h3 className="mb-2 font-semibold">Run details</h3>
          {(detail.run?.results ?? []).filter((r) => r.status !== 'PASSED').length === 0 ? (
            <p className="text-sm text-green-700">All {detail.passed} tests passed.</p>
          ) : (
            <ul className="text-sm">
              {(detail.run?.results ?? []).filter((r) => r.status !== 'PASSED').map((r) => (
                <li key={r.case_id}>{r.status} {r.case_id}: {r.message}</li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}
