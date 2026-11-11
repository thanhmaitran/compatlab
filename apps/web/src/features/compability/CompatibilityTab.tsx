import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { Async, EmptyState } from '../../components/States'
import { useAsync } from '../../hooks/useAsync'
import { formatDate } from '../../lib/format'
import { analyze, listAnalyses } from '../../services/compatibility-service'
import { listVersions } from '../../services/project-service'

export function CompatibilityTab({ projectId }: { projectId: string }) {
  const versions = useAsync(() => listVersions(projectId), [projectId])
  const analyses = useAsync(() => listAnalyses(projectId), [projectId])
  const [base, setBase] = useState('')
  const [target, setTarget] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const list = versions.data ?? []
  const labelOf = (id: string) => list.find((v) => v.id === id)?.label ?? id

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')
    if (!base || !target) return setError('Choose a base and a target version.')
    if (base === target) return setError('The base and target versions must differ.')
    if (!window.confirm(`Compare ${labelOf(base)} with ${labelOf(target)}?`)) return
    setBusy(true)
    try {
      await analyze(projectId, base, target)
      analyses.reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Analysis failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-semibold">Compatibility analysis</h2>
      <form onSubmit={submit} className="card grid gap-4 md:grid-cols-3" noValidate>
        <div>
          <label htmlFor="base-version" className="label">Base version</label>
          <select id="base-version" className="input" value={base} onChange={(e) => setBase(e.target.value)}>
            <option value="">Select…</option>
            {list.map((v) => <option key={v.id} value={v.id}>{v.label}</option>)}
          </select>
        </div>
        <div>
          <label htmlFor="target-version" className="label">Target version</label>
          <select id="target-version" className="input" value={target} onChange={(e) => setTarget(e.target.value)}>
            <option value="">Select…</option>
            {list.map((v) => <option key={v.id} value={v.id}>{v.label}</option>)}
          </select>
        </div>
        <div className="flex items-end">
          <button type="submit" className="btn" disabled={busy}>Run analysis</button>
        </div>
        {error && <p role="alert" className="text-sm text-red-700 md:col-span-3">{error}</p>}
      </form>
      <Async state={analyses} isEmpty={(l) => l.length === 0} empty={<EmptyState title="No analyses yet" hint="Pick two versions and run an analysis." />}>
        {(rows) => (
          <table className="card w-full text-left text-sm">
            <thead className="text-slate-500">
              <tr><th>Date</th><th>Comparison</th><th>Backward score</th><th>Backward changes</th><th /></tr>
            </thead>
            <tbody>
              {rows.map((a) => (
                <tr key={a.id} className="border-t border-slate-100">
                  <td className="py-2">{formatDate(a.created_at)}</td>
                  <td>{labelOf(a.base_version_id)} → {labelOf(a.target_version_id)}</td>
                  <td>{a.scores ? a.scores.backward : '–'}</td>
                  <td>{a.backward.breaking} breaking, {a.backward.conditional} conditional, {a.backward.non_breaking} non-breaking</td>
                  <td><Link className="text-indigo-700" to={`/projects/${projectId}/analyses/${a.id}`}>View</Link></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Async>
    </div>
  )
}
