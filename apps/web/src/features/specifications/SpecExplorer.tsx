import { Async } from '../../components/States'
import { useAsync } from '../../hooks/useAsync'
import { inspectVersion } from '../../services/project-service'

/** Read-only view of the paths, operations and schemas of one stored version. */
export function SpecExplorer({ versionId }: { versionId: string }) {
  const state = useAsync(() => inspectVersion(versionId), [versionId])
  return (
    <Async state={state}>
      {(spec) => (
        <div className="card space-y-4">
          <h3 className="font-semibold">
            {spec.title} {spec.api_version} <span className="text-sm font-normal text-slate-500">(OpenAPI {spec.openapi_version})</span>
          </h3>
          <table className="w-full text-left text-sm">
            <thead className="text-slate-500">
              <tr><th className="py-1">Method</th><th>Path</th><th>Parameters</th><th>Responses</th><th>Security</th></tr>
            </thead>
            <tbody>
              {spec.endpoints.map((e) => (
                <tr key={`${e.method} ${e.path}`} className="border-t border-slate-100 align-top">
                  <td className="py-1 font-mono">{e.method}</td>
                  <td className="font-mono">{e.path}</td>
                  <td>{e.parameters.map((p) => `${p.name} (${p.location}${p.required ? ', required' : ''})`).join(', ') || '–'}</td>
                  <td>{e.responses.join(', ') || '–'}</td>
                  <td>{e.security.join(', ') || '–'}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <div>
            <h4 className="mb-1 text-sm font-semibold">Schemas</h4>
            <ul className="space-y-1 text-sm">
              {spec.schemas.map((s) => (
                <li key={s.name}>
                  <span className="font-mono font-medium">{s.name}</span> <span className="text-slate-500">{s.type}</span>
                  {s.properties.length > 0 && <span>: {s.properties.map((p) => (s.required.includes(p) ? `${p}*` : p)).join(', ')}</span>}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </Async>
  )
}
