import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ExternalLink, RefreshCw, Trash2 } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { formatMs, formatRelative } from '../lib/format'
import type { Monitor } from '../types'
import { StatusBadge } from './StatusBadge'

export function MonitorCard({ monitor }: { monitor: Monitor }) {
  const qc = useQueryClient()

  const toggle = useMutation({
    mutationFn: () => api.updateMonitor(monitor.id, { enabled: !monitor.enabled }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['monitors'] })
      void qc.invalidateQueries({ queryKey: ['stats'] })
    },
  })

  const check = useMutation({
    mutationFn: () => api.checkMonitor(monitor.id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['monitors'] })
      void qc.invalidateQueries({ queryKey: ['history', monitor.id] })
      void qc.invalidateQueries({ queryKey: ['stats'] })
    },
  })

  const remove = useMutation({
    mutationFn: () => api.deleteMonitor(monitor.id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['monitors'] })
      void qc.invalidateQueries({ queryKey: ['stats'] })
    },
  })

  return (
    <article className="card flex flex-col gap-4 p-5 transition hover:border-accent/25">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <Link to={`/monitors/${monitor.id}`} className="truncate text-base font-semibold text-white hover:text-accent">
            {monitor.name}
          </Link>
          <a
            href={monitor.url}
            target="_blank"
            rel="noreferrer"
            className="mt-1 flex items-center gap-1 truncate font-mono text-xs text-slate-400 hover:text-slate-200"
          >
            <span className="truncate">{monitor.url}</span>
            <ExternalLink className="h-3 w-3 shrink-0" />
          </a>
        </div>
        <StatusBadge status={monitor.status} />
      </div>

      <div className="grid grid-cols-3 gap-3 text-sm">
        <div>
          <div className="text-[11px] uppercase tracking-wide text-slate-500">HTTP</div>
          <div className="font-mono text-slate-200">{monitor.last_status_code ?? '—'}</div>
        </div>
        <div>
          <div className="text-[11px] uppercase tracking-wide text-slate-500">Latency</div>
          <div className="font-mono text-slate-200">{formatMs(monitor.last_response_time_ms)}</div>
        </div>
        <div>
          <div className="text-[11px] uppercase tracking-wide text-slate-500">Checked</div>
          <div className="text-slate-200">{formatRelative(monitor.last_checked_at)}</div>
        </div>
      </div>

      {monitor.last_error && monitor.status === 'DOWN' ? (
        <p className="truncate rounded-lg bg-rose-500/10 px-2.5 py-1.5 text-xs text-rose-300">{monitor.last_error}</p>
      ) : null}

      <div className="mt-auto flex flex-wrap items-center gap-2 border-t border-white/5 pt-4">
        <label className="flex cursor-pointer items-center gap-2 text-xs text-slate-300">
          <input
            type="checkbox"
            className="h-4 w-4 rounded border-white/20 bg-surface-850 text-accent focus:ring-accent"
            checked={monitor.enabled}
            disabled={toggle.isPending}
            onChange={() => toggle.mutate()}
          />
          {monitor.enabled ? 'Enabled' : 'Disabled'}
        </label>
        <div className="ml-auto flex gap-2">
          <button
            type="button"
            className="btn-ghost"
            disabled={check.isPending}
            onClick={() => check.mutate()}
            title="Run check now"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${check.isPending ? 'animate-spin' : ''}`} />
            Check
          </button>
          <button
            type="button"
            className="btn-danger"
            disabled={remove.isPending}
            onClick={() => {
              if (confirm(`Delete monitor “${monitor.name}”?`)) remove.mutate()
            }}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    </article>
  )
}
