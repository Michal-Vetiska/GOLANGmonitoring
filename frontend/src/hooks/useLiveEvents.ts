import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { eventsUrl } from '../lib/api'
import type { CheckResult, Monitor, SSEEvent } from '../types'

/** Subscribes to SSE and patches TanStack Query caches in place. */
export function useLiveEvents(enabled = true) {
  const qc = useQueryClient()
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    if (!enabled) return

    let closed = false
    let retryTimer: ReturnType<typeof setTimeout> | undefined

    const connect = () => {
      if (closed) return
      const es = new EventSource(eventsUrl())
      esRef.current = es

      es.onmessage = (msg) => {
        try {
          const event = JSON.parse(msg.data) as SSEEvent
          handleEvent(event)
        } catch {
          /* ignore malformed */
        }
      }

      es.onerror = () => {
        es.close()
        esRef.current = null
        if (!closed) {
          retryTimer = setTimeout(connect, 2000)
        }
      }
    }

    const handleEvent = (event: SSEEvent) => {
      if (event.type === 'check_result') {
        const result = event.payload as CheckResult
        qc.setQueryData<Monitor[]>(['monitors'], (old) =>
          old?.map((m) =>
            m.id === result.monitor_id
              ? {
                  ...m,
                  status: result.is_up ? 'UP' : 'DOWN',
                  last_status_code: result.status_code ?? null,
                  last_response_time_ms: result.response_time_ms ?? null,
                  last_checked_at: result.checked_at,
                  last_error: result.error_message ?? null,
                  last_is_up: result.is_up,
                }
              : m,
          ),
        )
        qc.setQueryData<Monitor>(['monitor', result.monitor_id], (old) =>
          old
            ? {
                ...old,
                status: result.is_up ? 'UP' : 'DOWN',
                last_status_code: result.status_code ?? null,
                last_response_time_ms: result.response_time_ms ?? null,
                last_checked_at: result.checked_at,
                last_error: result.error_message ?? null,
                last_is_up: result.is_up,
              }
            : old,
        )
        qc.setQueryData<CheckResult[]>(['history', result.monitor_id], (old) => {
          if (!old) return old
          return [result, ...old].slice(0, 100)
        })
        void qc.invalidateQueries({ queryKey: ['stats'] })
        void qc.invalidateQueries({ queryKey: ['monitor-stats', result.monitor_id] })
      }

      if (event.type === 'monitor_update') {
        const monitor = event.payload as Monitor
        qc.setQueryData<Monitor[]>(['monitors'], (old) => {
          if (!old) return [monitor]
          const idx = old.findIndex((m) => m.id === monitor.id)
          if (idx === -1) return [...old, monitor]
          const next = [...old]
          next[idx] = { ...next[idx], ...monitor }
          return next
        })
        qc.setQueryData(['monitor', monitor.id], (old: Monitor | undefined) =>
          old ? { ...old, ...monitor } : monitor,
        )
        void qc.invalidateQueries({ queryKey: ['stats'] })
      }

      if (event.type === 'monitor_delete') {
        const { id } = event.payload as { id: string }
        qc.setQueryData<Monitor[]>(['monitors'], (old) => old?.filter((m) => m.id !== id))
        qc.removeQueries({ queryKey: ['monitor', id] })
        void qc.invalidateQueries({ queryKey: ['stats'] })
      }
    }

    connect()

    return () => {
      closed = true
      if (retryTimer) clearTimeout(retryTimer)
      esRef.current?.close()
      esRef.current = null
    }
  }, [enabled, qc])
}
