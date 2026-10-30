import { useQuery } from '@tanstack/react-query'
import { Activity, Clock3, Gauge, ShieldAlert, Wifi } from 'lucide-react'
import { AddMonitorForm } from '../components/AddMonitorForm'
import { MonitorCard } from '../components/MonitorCard'
import { StatCard } from '../components/StatCard'
import { api } from '../lib/api'
import { formatMs, formatPct } from '../lib/format'

export function DashboardPage() {
  const monitors = useQuery({ queryKey: ['monitors'], queryFn: api.listMonitors })
  const stats = useQuery({ queryKey: ['stats'], queryFn: api.getDashboardStats })

  return (
    <div className="space-y-8">
      <section className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-white sm:text-3xl">Dashboard</h1>
          <p className="mt-1 max-w-xl text-sm text-slate-400">
            Watch public HTTP endpoints in real time. Checks run every 30 seconds by default through a bounded Go worker pool.
          </p>
        </div>
        <AddMonitorForm />
      </section>

      <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <StatCard label="Total" value={String(stats.data?.total_monitors ?? '—')} icon={Activity} />
        <StatCard label="Online" value={String(stats.data?.online_monitors ?? '—')} icon={Wifi} hint="Latest check UP" />
        <StatCard label="Offline" value={String(stats.data?.offline_monitors ?? '—')} icon={ShieldAlert} hint="Latest check DOWN" />
        <StatCard label="Avg latency" value={formatMs(stats.data?.avg_response_time_ms)} icon={Clock3} />
        <StatCard label="Uptime" value={formatPct(stats.data?.overall_uptime_pct)} icon={Gauge} hint="Across stored history" />
      </section>

      {monitors.isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="card h-48 animate-pulse bg-surface-900/60" />
          ))}
        </div>
      ) : null}

      {monitors.isError ? (
        <div className="card border-rose-500/30 p-6 text-rose-300">
          Failed to load monitors: {(monitors.error as Error).message}
        </div>
      ) : null}

      {monitors.data && monitors.data.length === 0 ? (
        <div className="card flex flex-col items-center gap-3 border-dashed p-12 text-center">
          <Activity className="h-8 w-8 text-accent/70" />
          <h2 className="text-lg font-medium text-white">No monitors yet</h2>
          <p className="max-w-md text-sm text-slate-400">
            Add a public HTTPS endpoint to start collecting latency and availability data.
          </p>
        </div>
      ) : null}

      {monitors.data && monitors.data.length > 0 ? (
        <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {monitors.data.map((m) => (
            <MonitorCard key={m.id} monitor={m} />
          ))}
        </section>
      ) : null}
    </div>
  )
}
