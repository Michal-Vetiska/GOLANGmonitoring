export type MonitorStatus = 'UP' | 'DOWN' | 'UNKNOWN'

export interface Monitor {
  id: string
  name: string
  url: string
  interval_seconds: number
  enabled: boolean
  created_at: string
  updated_at: string
  status: MonitorStatus
  last_status_code?: number | null
  last_response_time_ms?: number | null
  last_checked_at?: string | null
  last_error?: string | null
  last_is_up?: boolean | null
}

export interface CheckResult {
  id: number
  monitor_id: string
  checked_at: string
  response_time_ms?: number | null
  status_code?: number | null
  is_up: boolean
  error_message?: string | null
}

export interface DashboardStats {
  total_monitors: number
  online_monitors: number
  offline_monitors: number
  unknown_monitors: number
  avg_response_time_ms: number
  overall_uptime_pct: number
}

export interface MonitorStats {
  uptime_pct: number
  avg_response_time_ms: number
  min_response_time_ms?: number | null
  max_response_time_ms?: number | null
  total_checks: number
  successful_checks: number
}

export interface CreateMonitorInput {
  name: string
  url: string
  interval_seconds?: number
  enabled?: boolean
}

export interface UpdateMonitorInput {
  name?: string
  url?: string
  interval_seconds?: number
  enabled?: boolean
}

export interface SSEEvent {
  type: 'check_result' | 'monitor_update' | 'monitor_delete'
  payload: unknown
}
