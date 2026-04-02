const BASE = '/api/v1'

function token(): string | null {
  return localStorage.getItem('sqzarr_token')
}

function headers(): HeadersInit {
  const t = token()
  return {
    'Content-Type': 'application/json',
    ...(t ? { Authorization: `Bearer ${t}` } : {}),
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method,
    headers: headers(),
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (res.status === 401) {
    localStorage.removeItem('sqzarr_token')
    window.location.href = '/login'
    throw new Error('Unauthorized')
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

// ---- Types ----

export interface Status {
  version: string
  encoder: string
  paused: boolean
  total_saved_gb: number
  jobs_done: number
  jobs_failed: number
  disk_free_gb: number
  pause_threshold: number
}

export interface Job {
  ID: number
  SourcePath: string
  SourceSize: number
  SourceCodec: string
  SourceDuration: number
  SourceBitrate: number
  Status: 'pending' | 'running' | 'done' | 'failed' | 'cancelled' | 'skipped'
  Progress: number
  BytesSaved: { Int64: number; Valid: boolean } | null
  ErrorMessage: { String: string; Valid: boolean } | null
  EncoderUsed: { String: string; Valid: boolean } | null
  StartedAt: { Time: string; Valid: boolean } | null
  FinishedAt: { Time: string; Valid: boolean } | null
  CreatedAt: string
}

export interface Directory {
  ID: number
  Path: string
  Enabled: boolean
  MinAgeDays: number
  MaxBitrate: number
  MinSizeMB: number
  CreatedAt: string
  UpdatedAt: string
}

// Input type for create/update — matches Go JSON struct tags (snake_case)
export interface DirectoryInput {
  path?: string
  enabled?: boolean
  min_age_days?: number
  max_bitrate?: number
  min_size_mb?: number
}

export interface Stats {
  TotalBytesSaved: number
  TotalJobsDone: number
  TotalJobsFailed: number
  UpdatedAt: string
}

// ---- API calls ----

export const api = {
  getStatus: () => request<Status>('GET', '/status'),
  getStats: () => request<Stats>('GET', '/stats'),

  listJobs: (status?: string, limit = 50, offset = 0) =>
    request<Job[]>('GET', `/jobs?${new URLSearchParams({
      ...(status ? { status } : {}),
      limit: String(limit),
      offset: String(offset),
    })}`),
  getJob: (id: number) => request<Job>('GET', `/jobs/${id}`),
  createJob: (path: string) => request<Job>('POST', '/jobs', { path }),
  cancelJob: (id: number) => request<void>('DELETE', `/jobs/${id}`),
  retryJob: (id: number) => request<void>('POST', `/jobs/${id}/retry`),

  listDirectories: () => request<Directory[]>('GET', '/directories'),
  createDirectory: (d: DirectoryInput) => request<Directory>('POST', '/directories', d),
  updateDirectory: (id: number, d: DirectoryInput) =>
    request<Directory>('PUT', `/directories/${id}`, d),
  deleteDirectory: (id: number) => request<void>('DELETE', `/directories/${id}`),

  triggerScan: () => request<{ status: string }>('POST', '/scan'),
  pauseQueue: () => request<{ paused: boolean }>('POST', '/queue/pause'),
  resumeQueue: () => request<{ paused: boolean }>('POST', '/queue/resume'),

  login: (password: string) =>
    request<{ token: string }>('POST', '/auth/login', { password }),
}

// ---- SSE ----

export type SSEEvent = {
  Type: 'progress' | 'done' | 'failed' | 'paused'
  JobID: number
  Progress: number
  Error?: string
}

export function connectSSE(onEvent: (e: SSEEvent) => void): () => void {
  const t = token()
  const url = `${BASE}/ws${t ? `?token=${encodeURIComponent(t)}` : ''}`
  let es: EventSource
  let closed = false

  function connect() {
    if (closed) return
    es = new EventSource(url)
    es.onmessage = (e) => {
      try {
        onEvent(JSON.parse(e.data))
      } catch {}
    }
    es.onerror = () => {
      es.close()
      if (!closed) setTimeout(connect, 3000)
    }
  }

  connect()
  return () => {
    closed = true
    es?.close()
  }
}
