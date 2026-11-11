import { useState } from 'react'
import type { ChangeEvent, FormEvent } from 'react'
import { Async, EmptyState } from '../../components/States'
import { useAsync } from '../../hooks/useAsync'
import { formatDate } from '../../lib/format'
import { listVersions, uploadSpecification } from '../../services/project-service'
import { SpecExplorer } from '../specifications/SpecExplorer'

const MAX_FILE_BYTES = 5 * 1024 * 1024

export function VersionsTab({ projectId }: { projectId: string }) {
  const versions = useAsync(() => listVersions(projectId), [projectId])
  const [label, setLabel] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [explored, setExplored] = useState('')

  function pick(e: ChangeEvent<HTMLInputElement>) {
    setFile(e.target.files?.[0] ?? null)
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')
    setMessage('')
    if (label.trim() === '') return setError('Enter a version label.')
    if (!file) return setError('Choose an OpenAPI file (.yaml, .yml or .json).')
    if (file.size > MAX_FILE_BYTES) return setError('The file is larger than 5 MiB.')
    setBusy(true)
    try {
      const res = await uploadSpecification(projectId, label.trim(), file.name, await file.text())
      setMessage(`Stored ${res.version.label}: ${res.endpoints} endpoints, ${res.schemas} schemas.`)
      setLabel('')
      versions.reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-6">
      <form onSubmit={submit} className="card grid gap-4 md:grid-cols-3" noValidate>
        <div>
          <label htmlFor="version-label" className="label">Version label</label>
          <input id="version-label" className="input" value={label} onChange={(e) => setLabel(e.target.value)} />
        </div>
        <div>
          <label htmlFor="spec-file" className="label">OpenAPI file</label>
          <input id="spec-file" type="file" accept=".yaml,.yml,.json" className="input" onChange={pick} />
        </div>
        <div className="flex items-end">
          <button type="submit" className="btn" disabled={busy}>Upload version</button>
        </div>
        {error && <p role="alert" className="text-sm text-red-700 md:col-span-3">{error}</p>}
        {message && <p role="status" className="text-sm text-green-700 md:col-span-3">{message}</p>}
      </form>
      <Async state={versions} isEmpty={(l) => l.length === 0} empty={<EmptyState title="No versions yet" hint="Upload an OpenAPI document to create the first version." />}>
        {(list) => (
          <ul className="space-y-2">
            {list.map((v) => (
              <li key={v.id} className="card flex items-center justify-between">
                <span className="font-medium">{v.label}</span>
                <span className="flex items-center gap-4">
                  <span className="text-xs text-slate-500">{formatDate(v.created_at)}</span>
                  <button type="button" className="btn-secondary" onClick={() => setExplored(explored === v.id ? '' : v.id)}>
                    {explored === v.id ? 'Hide' : 'Explore'}
                  </button>
                </span>
              </li>
            ))}
          </ul>
        )}
      </Async>
      {explored && <SpecExplorer versionId={explored} />}
    </div>
  )
}
