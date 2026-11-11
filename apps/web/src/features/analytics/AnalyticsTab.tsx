import type { ReactNode } from 'react'
import { Bar, BarChart, CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { DecisionBadge } from '../../components/Badge'
import { Async, EmptyState } from '../../components/States'
import { useAsync } from '../../hooks/useAsync'
import { formatDate } from '../../lib/format'
import { getAnalytics } from '../../services/analytics-service'

/** Each chart answers one question, stated in its title. */
function ChartCard({ question, children }: { question: string; children: ReactNode }) {
  return (
    <div className="card">
      <h3 className="mb-2 text-sm font-semibold">{question}</h3>
      <div className="h-64">{children}</div>
    </div>
  )
}

export function AnalyticsTab({ projectId }: { projectId: string }) {
  const state = useAsync(() => getAnalytics(projectId), [projectId])
  return (
    <Async state={state} isEmpty={(a) => a.total_analyses === 0} empty={<EmptyState title="No analytics yet" hint="Run at least one analysis to see compatibility trends." />}>
      {(a) => {
        const scores = a.score_trend.map((p) => ({ name: p.target, backward: p.backward, forward: p.forward }))
        const distribution = [
          { name: 'Breaking', count: a.distribution.breaking },
          { name: 'Conditional', count: a.distribution.conditional },
          { name: 'Non-breaking', count: a.distribution.non_breaking },
        ]
        const passRates = a.pass_rate_trend.map((p) => ({ name: formatDate(p.created_at), passRate: Number(p.pass_rate.toFixed(1)) }))
        const contracts = a.contract_trend.map((p) => ({ name: formatDate(p.created_at), honoured: p.total === 0 ? 0 : Math.round((p.passed / p.total) * 100) }))
        return (
          <div className="grid gap-4 md:grid-cols-2">
            <ChartCard question="Is backward and forward compatibility improving?">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={scores}>
                  <CartesianGrid strokeDasharray="3 3" /><XAxis dataKey="name" /><YAxis domain={[0, 100]} /><Tooltip /><Legend />
                  <Line type="monotone" dataKey="backward" stroke="#4f46e5" />
                  <Line type="monotone" dataKey="forward" stroke="#0891b2" />
                </LineChart>
              </ResponsiveContainer>
            </ChartCard>
            <ChartCard question="Which version introduced the most breaking changes? (backward)">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={a.breaking_by_version}>
                  <CartesianGrid strokeDasharray="3 3" /><XAxis dataKey="target" /><YAxis allowDecimals={false} /><Tooltip /><Legend />
                  <Bar dataKey="breaking" stackId="c" fill="#dc2626" />
                  <Bar dataKey="conditional" stackId="c" fill="#d97706" />
                  <Bar dataKey="non_breaking" stackId="c" fill="#16a34a" />
                </BarChart>
              </ResponsiveContainer>
            </ChartCard>
            <ChartCard question="How are changes classified overall? (backward)">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={distribution}>
                  <CartesianGrid strokeDasharray="3 3" /><XAxis dataKey="name" /><YAxis allowDecimals={false} /><Tooltip />
                  <Bar dataKey="count" fill="#4f46e5" />
                </BarChart>
              </ResponsiveContainer>
            </ChartCard>
            <ChartCard question="Is test quality improving?">
              {passRates.length === 0 ? <EmptyState title="No test runs yet" /> : (
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={passRates}>
                    <CartesianGrid strokeDasharray="3 3" /><XAxis dataKey="name" /><YAxis domain={[0, 100]} /><Tooltip />
                    <Line type="monotone" dataKey="passRate" stroke="#16a34a" />
                  </LineChart>
                </ResponsiveContainer>
              )}
            </ChartCard>
            <ChartCard question="Are consumer contracts still honoured? (% of interactions)">
              {contracts.length === 0 ? <EmptyState title="No contract verifications yet" /> : (
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={contracts}>
                    <CartesianGrid strokeDasharray="3 3" /><XAxis dataKey="name" /><YAxis domain={[0, 100]} /><Tooltip />
                    <Line type="monotone" dataKey="honoured" stroke="#7c3aed" />
                  </LineChart>
                </ResponsiveContainer>
              )}
            </ChartCard>
            <div className="card">
              <h3 className="mb-2 text-sm font-semibold">How often does the release gate block? ({a.gate_blocks} of {a.gate_history.length})</h3>
              {a.gate_history.length === 0 ? <EmptyState title="No gate evaluations yet" /> : (
                <ul className="space-y-1 text-sm">
                  {a.gate_history.map((g) => <li key={g.report_id}>{formatDate(g.created_at)} <DecisionBadge decision={g.decision} /></li>)}
                </ul>
              )}
            </div>
            <div className="card md:col-span-2">
              <h3 className="mb-2 text-sm font-semibold">Which operations change most often? (backward)</h3>
              <table className="w-full text-left text-sm">
                <thead className="text-slate-500"><tr><th>Operation</th><th>Changes</th></tr></thead>
                <tbody>
                  {a.top_changed_endpoints.map((e) => (
                    <tr key={`${e.method} ${e.path}`} className="border-t border-slate-100"><td className="py-1 font-mono">{e.method} {e.path}</td><td>{e.count}</td></tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )
      }}
    </Async>
  )
}
