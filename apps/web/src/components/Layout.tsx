import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../features/auth/AuthContext'

const links = [
  { to: '/', label: 'Overview', end: true },
  { to: '/projects', label: 'Projects', end: false },
  { to: '/settings', label: 'Settings', end: false },
]

export function Layout() {
  const { signOut } = useAuth()
  const navigate = useNavigate()
  return (
    <div className="flex min-h-screen">
      <aside className="w-56 shrink-0 bg-slate-900 p-4 text-slate-100">
        <h1 className="mb-6 text-lg font-bold">CompatLab</h1>
        <nav className="space-y-1">
          {links.map((l) => (
            <NavLink
              key={l.to}
              to={l.to}
              end={l.end}
              className={({ isActive }) => `block rounded px-3 py-2 text-sm ${isActive ? 'bg-indigo-600' : 'hover:bg-slate-800'}`}
            >
              {l.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <div className="flex-1">
        <header className="flex justify-end border-b border-slate-200 bg-white px-6 py-3">
          <button
            type="button"
            className="btn-secondary"
            onClick={() => {
              signOut()
              navigate('/login')
            }}
          >
            Sign out
          </button>
        </header>
        <main className="mx-auto max-w-6xl p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
