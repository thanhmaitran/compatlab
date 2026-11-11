import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { Async, EmptyState } from '../../components/States'
import { useAsync } from '../../hooks/useAsync'
import { formatDate } from '../../lib/format'
import { createProject, listProjects } from '../../services/project-service'

export function ProjectsPage() {
  const projects = useAsync(listProjects, [])
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')
    if (name.trim().length < 2) return setError('Project name must have at least 2 characters.')
    setBusy(true)
    try {
      await createProject(name.trim(), description.trim())
      setName('')
      setDescription('')
      projects.reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Request failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Projects</h1>
      <form onSubmit={submit} className="card grid gap-4 md:grid-cols-3" noValidate>
        <div>
          <label htmlFor="project-name" className="label">Project name</label>
          <input id="project-name" className="input" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div>
          <label htmlFor="project-description" className="label">Description</label>
          <input id="project-description" className="input" value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <div className="flex items-end">
          <button type="submit" className="btn" disabled={busy}>Create project</button>
        </div>
        {error && <p role="alert" className="text-sm text-red-700 md:col-span-3">{error}</p>}
      </form>
      <Async state={projects} isEmpty={(l) => l.length === 0} empty={<EmptyState title="No projects yet" hint="Create your first API project above." />}>
        {(list) => (
          <ul className="space-y-2">
            {list.map((p) => (
              <li key={p.id} className="card flex items-center justify-between">
                <div>
                  <Link className="font-medium text-indigo-700" to={`/projects/${p.id}`}>{p.name}</Link>
                  {p.description && <p className="text-sm text-slate-500">{p.description}</p>}
                </div>
                <span className="text-xs text-slate-500">{formatDate(p.created_at)}</span>
              </li>
            ))}
          </ul>
        )}
      </Async>
    </div>
  )
}
