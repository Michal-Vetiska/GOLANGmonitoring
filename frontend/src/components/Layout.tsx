import { Activity } from 'lucide-react'
import { Link, Outlet } from 'react-router-dom'

export function Layout() {
  return (
    <div className="mx-auto flex min-h-screen max-w-7xl flex-col px-4 py-6 sm:px-6 lg:px-8">
      <header className="mb-8 flex items-center justify-between gap-4">
        <Link to="/" className="group flex items-center gap-3">
          <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-accent/15 ring-1 ring-accent/30 transition group-hover:bg-accent/25">
            <Activity className="h-5 w-5 text-accent" strokeWidth={2.25} />
          </span>
          <div>
            <div className="text-lg font-semibold tracking-tight text-white">PulseWatch</div>
            <div className="text-xs text-slate-400">Real-time API monitoring</div>
          </div>
        </Link>
        <div className="hidden items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1.5 text-xs text-slate-400 sm:flex">
          <span className="status-dot animate-pulse bg-accent" />
          Live SSE connected
        </div>
      </header>
      <main className="flex-1">
        <Outlet />
      </main>
      <footer className="mt-10 border-t border-white/5 pt-4 text-center text-xs text-slate-500">
        PulseWatch · portfolio project · Go + React
      </footer>
    </div>
  )
}
