import type {
  CheckResult,
  CreateMonitorInput,
  DashboardStats,
  Monitor,
  MonitorStats,
  UpdateMonitorInput,
} from '../types'

const BASE = import.meta.env.VITE_API_BASE ?? ''

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
    ...init,
  })
  if (!res.ok) {
    let message = res.statusText
    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      /* ignore */
    }
    throw new Error(message)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export const api = {
  health: () => request<{ status: string }>('/api/health'),
  listMonitors: () => request<Monitor[]>('/api/monitors'),
  getMonitor: (id: string) => request<Monitor>(`/api/monitors/${id}`),
  createMonitor: (input: CreateMonitorInput) =>
    request<Monitor>('/api/monitors', { method: 'POST', body: JSON.stringify(input) }),
  updateMonitor: (id: string, input: UpdateMonitorInput) =>
    request<Monitor>(`/api/monitors/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
  deleteMonitor: (id: string) =>
    request<void>(`/api/monitors/${id}`, { method: 'DELETE' }),
  checkMonitor: (id: string) =>
    request<CheckResult>(`/api/monitors/${id}/check`, { method: 'POST' }),
  getHistory: (id: string, limit = 100) =>
    request<CheckResult[]>(`/api/monitors/${id}/history?limit=${limit}`),
  getMonitorStats: (id: string) =>
    request<MonitorStats>(`/api/monitors/${id}/stats`),
  getDashboardStats: () => request<DashboardStats>('/api/stats'),
}

export function eventsUrl(): string {
  return `${BASE}/api/events`
}
