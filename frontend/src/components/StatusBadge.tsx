import type { MonitorStatus } from '../types'

const styles: Record<MonitorStatus, string> = {
  UP: 'bg-emerald-500/15 text-emerald-300 ring-emerald-500/30',
  DOWN: 'bg-rose-500/15 text-rose-300 ring-rose-500/30',
  UNKNOWN: 'bg-slate-500/15 text-slate-300 ring-slate-500/30',
}

const dots: Record<MonitorStatus, string> = {
  UP: 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.7)]',
  DOWN: 'bg-rose-400 shadow-[0_0_8px_rgba(251,113,133,0.7)]',
  UNKNOWN: 'bg-slate-400',
}

export function StatusBadge({ status }: { status: MonitorStatus }) {
  return (
    <span className={`inline-flex items-center gap-2 rounded-full px-2.5 py-1 text-xs font-medium ring-1 ${styles[status]}`}>
      <span className={`status-dot ${dots[status]}`} />
      {status}
    </span>
  )
}
