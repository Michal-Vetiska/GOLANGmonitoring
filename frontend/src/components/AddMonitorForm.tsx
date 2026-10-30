import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { api } from '../lib/api'

export function AddMonitorForm() {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [interval, setIntervalSec] = useState(30)
  const [error, setError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: () =>
      api.createMonitor({
        name: name.trim(),
        url: url.trim(),
        interval_seconds: interval,
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['monitors'] })
      void qc.invalidateQueries({ queryKey: ['stats'] })
      setName('')
      setUrl('')
      setIntervalSec(30)
      setError(null)
      setOpen(false)
    },
    onError: (err: Error) => setError(err.message),
  })

  if (!open) {
    return (
      <button type="button" className="btn-primary" onClick={() => setOpen(true)}>
        <Plus className="h-4 w-4" />
        Add monitor
      </button>
    )
  }

  return (
    <form
      className="card w-full max-w-xl space-y-3 p-4"
      onSubmit={(e) => {
        e.preventDefault()
        setError(null)
        mutation.mutate()
      }}
    >
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-white">New endpoint</h2>
        <button type="button" className="text-xs text-slate-400 hover:text-white" onClick={() => setOpen(false)}>
          Cancel
        </button>
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="block space-y-1.5 sm:col-span-1">
          <span className="text-xs text-slate-400">Name</span>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Payments API" required />
        </label>
        <label className="block space-y-1.5 sm:col-span-1">
          <span className="text-xs text-slate-400">Interval (seconds)</span>
          <input
            className="input"
            type="number"
            min={10}
            max={3600}
            value={interval}
            onChange={(e) => setIntervalSec(Number(e.target.value))}
          />
        </label>
        <label className="block space-y-1.5 sm:col-span-2">
          <span className="text-xs text-slate-400">URL</span>
          <input
            className="input font-mono text-[13px]"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://api.example.com/health"
            required
          />
        </label>
      </div>
      {error ? <p className="text-sm text-rose-300">{error}</p> : null}
      <div className="flex justify-end gap-2">
        <button type="submit" className="btn-primary" disabled={mutation.isPending}>
          {mutation.isPending ? 'Saving…' : 'Create monitor'}
        </button>
      </div>
    </form>
  )
}
