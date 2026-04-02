import { NavLink, Outlet } from 'react-router-dom'
import {
  LayoutDashboard,
  ListOrdered,
  Clock,
  FolderOpen,
  Settings,
} from 'lucide-react'
import { cn } from '../lib/utils'

const nav = [
  { to: '/',            label: 'Dashboard', icon: LayoutDashboard },
  { to: '/queue',       label: 'Queue',     icon: ListOrdered },
  { to: '/history',     label: 'History',   icon: Clock },
  { to: '/directories', label: 'Directories', icon: FolderOpen },
  { to: '/settings',    label: 'Settings',  icon: Settings },
]

export function Layout() {
  return (
    <div className="min-h-screen bg-stone-50 flex flex-col md:flex-row">
      {/* Sidebar — desktop */}
      <aside className="hidden md:flex flex-col w-56 bg-white border-r border-stone-200 shrink-0">
        <div className="px-5 py-5 border-b border-stone-200">
          <h1 className="text-lg font-semibold text-stone-900 tracking-tight">SQZARR</h1>
          <p className="text-xs text-stone-500 mt-0.5">Media Transcoder</p>
        </div>
        <nav className="flex-1 py-3">
          {nav.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 px-5 py-2.5 text-sm transition-colors',
                  isActive
                    ? 'text-stone-900 bg-stone-100 border-l-2 border-amber-500 font-medium'
                    : 'text-stone-600 hover:text-stone-900 hover:bg-stone-50 border-l-2 border-transparent',
                )
              }
            >
              <Icon size={16} />
              {label}
            </NavLink>
          ))}
        </nav>
      </aside>

      {/* Main content */}
      <main className="flex-1 flex flex-col min-w-0 pb-16 md:pb-0">
        <Outlet />
      </main>

      {/* Bottom tab bar — mobile */}
      <nav className="md:hidden fixed bottom-0 inset-x-0 bg-white border-t border-stone-200 flex z-50">
        {nav.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              cn(
                'flex-1 flex flex-col items-center gap-1 py-2 text-xs transition-colors',
                isActive ? 'text-amber-600' : 'text-stone-500',
              )
            }
          >
            <Icon size={20} />
            <span>{label}</span>
          </NavLink>
        ))}
      </nav>
    </div>
  )
}
