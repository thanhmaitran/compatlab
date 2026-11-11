import { useParams, useSearchParams } from 'react-router-dom'
import { Async } from '../../components/States'
import { useAsync } from '../../hooks/useAsync'
import { getProject } from '../../services/project-service'
import { AnalyticsTab } from '../analytics/AnalyticsTab'
import { CompatibilityTab } from '../compatibility/CompatibilityTab'
import { ContractsTab } from '../contracts/ContractsTab'
import { PoliciesTab } from '../policies/PoliciesTab'
import { TestingTab } from '../testing/TestingTab'
import { VersionsTab } from '../versions/VersionsTab'

const tabs = ['Versions', 'Compatibility', 'Contracts', 'Testing', 'Policies', 'Analytics'] as const
type Tab = (typeof tabs)[number]

export function ProjectPage() {
  const { id = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const requested = params.get('tab')
  const tab: Tab = tabs.find((t) => t === requested) ?? 'Versions'
  const project = useAsync(() => getProject(id), [id])

  return (
    <div className="space-y-6">
      <Async state={project}>
        {(p) => (
          <div>
            <h1 className="text-2xl font-bold">{p.name}</h1>
            {p.description && <p className="text-sm text-slate-500">{p.description}</p>}
          </div>
        )}
      </Async>
      <div role="tablist" className="flex gap-2 border-b border-slate-200">
        {tabs.map((t) => (
          <button
            key={t}
            type="button"
            role="tab"
            aria-selected={t === tab}
            className={`-mb-px border-b-2 px-3 py-2 text-sm ${t === tab ? 'border-indigo-600 font-semibold text-indigo-700' : 'border-transparent text-slate-600'}`}
            onClick={() => setParams({ tab: t })}
          >
            {t}
          </button>
        ))}
      </div>
      {tab === 'Versions' && <VersionsTab projectId={id} />}
      {tab === 'Compatibility' && <CompatibilityTab projectId={id} />}
      {tab === 'Contracts' && <ContractsTab projectId={id} />}
      {tab === 'Testing' && <TestingTab projectId={id} />}
      {tab === 'Policies' && <PoliciesTab projectId={id} />}
      {tab === 'Analytics' && <AnalyticsTab projectId={id} />}
    </div>
  )
}
