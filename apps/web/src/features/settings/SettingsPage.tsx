import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

export function SettingsPage() {
  const { signOut } = useAuth()
  const navigate = useNavigate()
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">Settings</h1>
      <div className="card space-y-2 text-sm">
        <p>API base path: <code>/api/v1</code> (proxied to the backend by the web server).</p>
        <p>The session token is kept in this browser tab only and is removed when you sign out or close the tab.</p>
        <button type="button" className="btn-secondary" onClick={() => { signOut(); navigate('/login') }}>Sign out</button>
      </div>
    </div>
  )
}
