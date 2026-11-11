import { useState } from 'react'
import type { FormEvent } from 'react'
import { Async, EmptyState } from '../../components/States'
import { Pill } from '../../components/Badge'
import { useAsync } from '../../hooks/useAsync'
import { listVersions } from '../../services/project-service'
import { listContracts, registerContract, verifyContract } from '../../services/contract-service'
import type { VerifyResult } from '../../types'

const example = `{
  "contract_version": "1.0",
  "consumer": "mobile-app",
  "provider": "users-api",
  "interactions": [
    {
      "description": "Retrieve user",
      "request": { "method": "GET", "path": "/users/u-123" },
      "response": { "status": 200, "body": { "id": "u-123", "email": "user@example.com" } }
    }
  ]
}`

export function ContractsTab({ projectId }: { projectId: string }) {
  const contracts = useAsync(() => listContracts(projectId), [projectId])
  const versions = useAsync(() => listVersions(projectId), [projectId])
  const [text, setText] = useState('')
  const [error, setError] = useState('')
  const [contractId, setContractId] = useState('')
  const [versionId, setVersionId] = useState('')
  const [result, setResult] = useState<VerifyResult | null>(null)
  const [verifyError, setVerifyError] = useState('')

  async function register(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')
    let parsed: unknown
    try {
      parsed = JSON.parse(text)
    } catch {
      return setError('The contract is not valid JSON.')
    }
    try {
      await registerContract(projectId, parsed)
      setText('')
      contracts.reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    }
  }

  async function verify(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setVerifyError('')
    setResult(null)
    if (!contractId || !versionId) return setVerifyError('Choose a contract and a version.')
    try {
      setResult(await verifyContract(contractId, versionId))
    } catch (err) {
      setVerifyError(err instanceof Error ? err.message : 'Verification failed')
    }
  }

  return (
    <div className="space-y-6">
      <form onSubmit={register} className="card space-y-3" noValidate>
        <label htmlFor="contract-json" className="label">Consumer contract (JSON)</label>
        <textarea id="contract-json" rows={8} className="input font-mono" value={text} onChange={(e) => setText(e.target.value)} />
        {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
        <div className="flex gap-2">
          <button type="submit" className="btn">Register contract</button>
          <button type="button" className="btn-secondary" onClick={() => setText(example)}>Load example</button>
        </div>
      </form>
      <Async state={contracts} isEmpty={(l) => l.length === 0} empty={<EmptyState title="No contracts yet" hint="Register what a consumer depends on to see who a change affects." />}>
        {(list) => (
          <>
            <table className="card w-full text-left text-sm">
              <thead className="text-slate-500"><tr><th>Consumer</th><th>Provider</th><th>Interactions</th></tr></thead>
              <tbody>
                {list.map((c) => (
                  <tr key={c.id} className="border-t border-slate-100"><td className="py-2">{c.consumer}</td><td>{c.provider}</td><td>{c.interactions}</td></tr>
                ))}
              </tbody>
            </table>
            <form onSubmit={verify} className="card grid gap-4 md:grid-cols-3" noValidate>
              <div>
                <label htmlFor="verify-contract" className="label">Contract</label>
                <select id="verify-contract" className="input" value={contractId} onChange={(e) => setContractId(e.target.value)}>
                  <option value="">Select…</option>
                  {list.map((c) => <option key={c.id} value={c.id}>{c.consumer}</option>)}
                </select>
              </div>
              <div>
                <label htmlFor="verify-version" className="label">Version to verify against</label>
                <select id="verify-version" className="input" value={versionId} onChange={(e) => setVersionId(e.target.value)}>
                  <option value="">Select…</option>
                  {(versions.data ?? []).map((v) => <option key={v.id} value={v.id}>{v.label}</option>)}
                </select>
              </div>
              <div className="flex items-end"><button type="submit" className="btn">Verify contract</button></div>
              {verifyError && <p role="alert" className="text-sm text-red-700 md:col-span-3">{verifyError}</p>}
            </form>
          </>
        )}
      </Async>
      {result && (
        <div className="card space-y-2" role="status">
          <p className="font-semibold">
            {result.verification.consumer} → {result.verification.provider}: <Pill tone={result.passed ? 'green' : 'red'}>{result.passed ? 'PASSED' : 'FAILED'}</Pill>
          </p>
          <ul className="text-sm">
            {result.verification.interactions.map((i, n) => (
              <li key={n}>{i.passed ? '✓' : '✗'} {i.description}{i.message && <span className="text-red-700"> — {i.message}</span>}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
