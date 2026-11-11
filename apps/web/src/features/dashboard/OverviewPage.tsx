import { Link } from 'react-router-dom'
import { DecisionBadge } from '../../components/Badge'
import { Async, EmptyState } from '../../components/States'
import { useAsync } from '../../hooks/useAsync'
import { formatPercent } from '../../lib/format'
import { getAnalytics } from '../../services/analytics-service'
import { listProjects } from '../../services/project-service'

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="card">
      <p className="text-sm text-slate-500">{label}</p>
      <p className="mt-1 text-2xl font-bold">{value}</p>
    </div>
  )
}

/** Every number on this page is loaded from the backend. */
export function OverviewPage() {
  const state = useAsync(async () => {
    const projects = await listProjects()
    const analytics = await Promise.all(projects.map((p) => getAnalytics(p.id)))
    return { projects, analytics }
  }, [])

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Overview</h1>
      <Async
        state={state}
        isEmpty={(d) => d.projects.length === 0}
        empty={<EmptyState title="No projects yet" hint="Create a project to start analysing API versions." />}
      >
        {({ projects, analytics }) => {
          const analyses = analytics.reduce((n, a) => n + a.total_analyses, 0)
          const breaking = analytics.reduce((n, a) => n + a.distribution.breaking, 0)
          const blocks = analytics.reduce((n, a) => n + a.gate_blocks, 0)
          const latest = analytics.flatMap((a) => a.pass_rate_trend.slice(-1).map((p) => p.pass_rate))
          const avg = latest.length > 0 ? formatPercent(latest.reduce((s, v) => s + v, 0) / latest.length) : '–'
          return (
            <>
              <div className="grid grid-cols-2 gap-4 md:grid-cols-5">
                <Stat label="API projects" value={String(projects.length)} />
                <Stat label="Analysis runs" value={String(analyses)} />
                <Stat label="Breaking changes (backward)" value={String(breaking)} />
                <Stat label="Blocked releases" value={String(blocks)} />
                <Stat label="Latest test pass rate" value={avg} />
              </div>
              <div className="card">
                <h2 className="mb-3 font-semibold">Projects</h2>
                <table className="w-full text-left text-sm">
                  <thead className="text-slate-500">
                    <tr><th className="py-1">Project</th><th>Analyses</th><th>Latest backward score</th><th>Latest gate</th></tr>
                  </thead>
                  <tbody>
                    {projects.map((p, i) => {
                      const a = analytics[i]
                      const score = a.score_trend.slice(-1)[0]
                      const gate = a.gate_history.slice(-1)[0]
                      return (
                        <tr key={p.id} className="border-t border-slate-100">
                          <td className="py-2"><Link className="text-indigo-700" to={`/projects/${p.id}`}>{p.name}</Link></td>
                          <td>{a.total_analyses}</td>
                          <td>{score ? score.backward : '–'}</td>
                          <td>{gate ? <DecisionBadge decision={gate.decision} /> : '–'}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </>
          )
        }}
      </Async>
    </div>
  )
}
