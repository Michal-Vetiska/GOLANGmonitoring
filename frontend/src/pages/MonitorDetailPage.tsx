import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, RefreshCw, Trash2 } from 'lucide-react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ResponseChart } from '../components/ResponseChart'
import { StatusBadge } from '../components/StatusBadge'
import { api } from '../lib/api'
import { formatMs, formatPct, formatRelative, formatTime } from '../lib/format'

export function MonitorDetailPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const qc = useQueryClient()

  const monitor = useQuery({ queryKey: ['monitor', id], queryFn: () => api.getMonitor(id), enabled: !!id })
  const history = useQuery({ queryKey: ['history', id], queryFn: () => api.getHistory(id, 100), enabled: !!id })
  const stats = useQuery({ queryKey: ['monitor-stats', id], queryFn: () => api.getMonitorStats(id), enabled: !!id })

  const check = useMutation({
    mutationFn: () => api.checkMonitor(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['monitor', id] })
      void qc.invalidateQueries({ queryKey: ['history', id] })
      void qc.invalidateQueries({ queryKey: ['monitor-stats', id] })
      void qc.invalidateQueries({ queryKey: ['monitors'] })
    },
  })

  const remove = useMutation({
    mutationFn: () => api.deleteMonitor(id),
    onSuccess: () => navigate('/'),
  })

  if (monitor.isLoading) {
    return <div className="card h-64 animate-pulse" />
  }

  if (monitor.isError || !monitor.data) {
    return (
      <div className="card p-8 text-center">
        <p className="text-rose-300">Monitor not found.</p>
        <Link to="/" className="btn-ghost mt-4 inline-flex">
          Back to dashboard
        </Link>
      </div>
    )
  }

  const m = monitor.data

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <Link to="/" className="mb-3 inline-flex items-center gap-1.5 text-sm text-slate-400 hover:text-white">
            <ArrowLeft className="h-4 w-4" />
            Dashboard
          </Link>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-2xl font-semibold text-white">{m.name}</h1>
            <StatusBadge status={m.status} />
          </div>
          <p className="mt-1 font-mono text-sm text-slate-400">{m.url}</p>
        </div>
        <div className="flex gap-2">
          <button type="button" className="btn-ghost" disabled={check.isPending} onClick={() => check.mutate()}>
            <RefreshCw className={`h-4 w-4 ${check.isPending ? 'animate-spin' : ''}`} />
            Check now
          </button>
          <button
            type="button"
            className="btn-danger"
            onClick={() => {
              if (confirm('Delete this monitor?')) remove.mutate()
            }}
          >
            <Trash2 className="h-4 w-4" />
            Delete
          </button>
        </div>
      </div>

      <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Metric label="HTTP status" value={String(m.last_status_code ?? '—')} />
        <Metric label="Latency" value={formatMs(m.last_response_time_ms)} />
        <Metric label="Last checked" value={formatRelative(m.last_checked_at)} />
        <Metric label="Interval" value={`${m.interval_seconds}s`} />
        <Metric label="Uptime" value={formatPct(stats.data?.uptime_pct)} />
        <Metric label="Avg latency" value={formatMs(stats.data?.avg_response_time_ms)} />
        <Metric
          label="Min / Max"
          value={`${formatMs(stats.data?.min_response_time_ms)} / ${formatMs(stats.data?.max_response_time_ms)}`}
        />
        <Metric label="Checks" value={`${stats.data?.successful_checks ?? 0} / ${stats.data?.total_checks ?? 0}`} />
      </section>

      {m.last_error ? (
        <div className="card border-rose-500/20 p-4 text-sm text-rose-300">
          <div className="mb-1 text-xs uppercase tracking-wide text-rose-400/80">Last error</div>
          {m.last_error}
        </div>
      ) : null}

      <section className="card p-5">
        <h2 className="mb-4 text-sm font-semibold text-white">Response time</h2>
        <ResponseChart history={history.data ?? []} />
      </section>

      <section className="card overflow-hidden">
        <div className="border-b border-white/5 px-5 py-4">
          <h2 className="text-sm font-semibold text-white">Recent checks</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[640px] text-left text-sm">
            <thead className="bg-white/[0.02] text-xs uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-5 py-3 font-medium">Time</th>
                <th className="px-5 py-3 font-medium">Status</th>
                <th className="px-5 py-3 font-medium">HTTP</th>
                <th className="px-5 py-3 font-medium">Latency</th>
                <th className="px-5 py-3 font-medium">Error</th>
              </tr>
            </thead>
            <tbody>
              {(history.data ?? []).length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-5 py-8 text-center text-slate-500">
                    No results yet.
                  </td>
                </tr>
              ) : (
                (history.data ?? []).map((r) => (
                  <tr key={r.id} className="border-t border-white/5 hover:bg-white/[0.02]">
                    <td className="px-5 py-3 font-mono text-xs text-slate-300">{formatTime(r.checked_at)}</td>
                    <td className="px-5 py-3">
                      <span className={r.is_up ? 'text-emerald-300' : 'text-rose-300'}>{r.is_up ? 'UP' : 'DOWN'}</span>
                    </td>
                    <td className="px-5 py-3 font-mono text-slate-300">{r.status_code ?? '—'}</td>
                    <td className="px-5 py-3 font-mono text-slate-300">{formatMs(r.response_time_ms)}</td>
                    <td className="max-w-xs truncate px-5 py-3 text-slate-400">{r.error_message ?? '—'}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="card p-4">
      <div className="text-[11px] uppercase tracking-wide text-slate-500">{label}</div>
      <div className="mt-1 text-lg font-medium text-white">{value}</div>
    </div>
  )
}
