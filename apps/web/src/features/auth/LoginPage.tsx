import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { isValidEmail } from '../../lib/format'
import { useAuth } from './AuthContext'

export function LoginPage() {
  const { signIn, signUp } = useAuth()
  const navigate = useNavigate()
  const [registering, setRegistering] = useState(false)
  const [organization, setOrganization] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')
    if (registering && organization.trim().length < 2) return setError('Organization name must have at least 2 characters.')
    if (!isValidEmail(email)) return setError('Enter a valid e-mail address.')
    if (password.length < 8) return setError('Password must have at least 8 characters.')
    setBusy(true)
    try {
      if (registering) await signUp(organization.trim(), email, password)
      else await signIn(email, password)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Request failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="mx-auto mt-24 max-w-sm">
      <form onSubmit={submit} className="card space-y-4" noValidate>
        <h1 className="text-xl font-bold">{registering ? 'Create account' : 'Sign in'}</h1>
        {registering && (
          <div>
            <label htmlFor="organization" className="label">Organization</label>
            <input id="organization" className="input" value={organization} onChange={(e) => setOrganization(e.target.value)} />
          </div>
        )}
        <div>
          <label htmlFor="email" className="label">Email</label>
          <input id="email" type="email" className="input" value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div>
          <label htmlFor="password" className="label">Password</label>
          <input id="password" type="password" className="input" value={password} onChange={(e) => setPassword(e.target.value)} />
        </div>
        {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
        <button type="submit" className="btn w-full" disabled={busy}>{registering ? 'Create account' : 'Sign in'}</button>
        <button type="button" className="w-full text-sm text-indigo-700" onClick={() => setRegistering(!registering)}>
          {registering ? 'Have an account? Sign in' : 'Need an account? Register'}
        </button>
      </form>
    </div>
  )
}
