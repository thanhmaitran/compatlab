import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ClassificationBadge } from '../../components/Badge'
import { Async } from '../../components/States'
import { useAsync } from '../../hooks/useAsync'
import { compareSeverity } from '../../lib/format'
import { getAnalysis } from '../../services/compatibility-service'
import type { Classification, Direction, Summary } from '../../types'

function SummaryCard({ title, summary }: { title: string; summary: Summary }) {
  return (
    <div className="card">
      <h3 className="mb-2 text-sm font-semibold">{title}</h3>
      <p className="text-sm">
        <strong>{summary.breaking}</strong> breaking · <strong>{summary.conditional}</strong> conditional · <strong>{summary.non_breaking}</strong> non-breaking
      </p>
    </div>
  )
}

export function DiffViewerPage() {
  const { id = '', analysisId = '' } = useParams()
  const state = useAsync(() => getAnalysis(analysisId), [analysisId])
  const [classification, setClassification] = useState<Classification | ''>('')
  const [direction, setDirection] = useState<Direction | ''>('BACKWARD')

  return (
    <div className="space-y-6">
      <Link className="text-sm text-indigo-700" to={`/projects/${id}?tab=Compatibility`}>← Back to project</Link>
      <h1 className="text-2xl font-bold">Analysis</h1>
      <Async state={state}>
        {(a) => {
          const changes = (a.analysis?.changes ?? [])
            .filter((c) => (classification === '' || c.classification === classification) && (direction === '' || c.direction === direction))
            .sort((x, y) => compareSeverity(x.severity, y.severity) || (x.location ?? '').localeCompare(y.location ?? ''))
          return (
            <>
              <p className="text-slate-600">
                {a.analysis?.base_version} → {a.analysis?.target_version}
                {a.scores && <span> · CompatLab score (not an industry standard): backward {a.scores.backward}, forward {a.scores.forward}</span>}
              </p>
              <div className="grid gap-4 md:grid-cols-2">
                <SummaryCard title="Backward: do existing consumers keep working?" summary={a.backward} />
                <SummaryCard title="Forward: do new consumers work with the old provider?" summary={a.forward} />
              </div>
              <div className="flex gap-4">
                <div>
                  <label htmlFor="filter-class" className="label">Classification</label>
                  <select id="filter-class" className="input" value={classification} onChange={(e) => setClassification(e.target.value as Classification | '')}>
                    <option value="">All</option>
                    <option value="BREAKING">BREAKING</option>
                    <option value="CONDITIONAL">CONDITIONAL</option>
                    <option value="NON_BREAKING">NON_BREAKING</option>
                  </select>
                </div>
                <div>
                  <label htmlFor="filter-direction" className="label">Direction</label>
                  <select id="filter-direction" className="input" value={direction} onChange={(e) => setDirection(e.target.value as Direction | '')}>
                    <option value="BACKWARD">BACKWARD</option>
                    <option value="FORWARD">FORWARD</option>
                    <option value="">All</option>
                  </select>
                </div>
              </div>
              <table className="card w-full text-left text-sm">
                <thead className="text-slate-500">
                  <tr><th>Class</th><th>Severity</th><th>Rule</th><th>Operation</th><th>Location</th><th>Change</th></tr>
                </thead>
                <tbody>
                  {changes.map((c, i) => (
                    <tr key={`${c.id}-${c.direction}-${i}`} className="border-t border-slate-100 align-top">
                      <td className="py-2"><ClassificationBadge value={c.classification} /></td>
                      <td>{c.severity}</td>
                      <td className="font-mono">{c.rule_id}</td>
                      <td className="font-mono">{c.method} {c.path}</td>
                      <td className="font-mono">{c.location}</td>
                      <td>
                        <details>
                          <summary>{c.description}</summary>
                          {c.explanation && <p className="mt-1 text-slate-600">{c.explanation}</p>}
                          {c.remediation && <p className="mt-1 text-slate-600">Remediation: {c.remediation}</p>}
                        </details>
                      </td>
                    </tr>
                  ))}
                  {changes.length === 0 && <tr><td colSpan={6} className="py-4 text-center text-slate-500">No changes match the filters.</td></tr>}
                </tbody>
              </table>
            </>
          )
        }}
      </Async>
    </div>
  )
}
