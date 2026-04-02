import { useEffect, useState, useCallback } from 'react'
import { api, type Job } from '../lib/api'
import { useSSE } from '../hooks/useSSE'
import { Card } from '../components/Card'
import { StatusBadge } from '../components/StatusBadge'
import { ProgressBar } from '../components/ProgressBar'
import { formatBytes, formatDuration, basename } from '../lib/utils'
import { X } from 'lucide-react'

export function Queue() {
  const [running, setRunning] = useState<Job[]>([])
  const [pending, setPending] = useState<Job[]>([])

  const load = useCallback(async () => {
    try {
      const [r, p] = await Promise.all([
        api.listJobs('running', 10, 0),
        api.listJobs('pending', 50, 0),
      ])
      setRunning(r ?? [])
      setPending(p ?? [])
    } catch {}
  }, [])

  useEffect(() => { load() }, [load])

  useSSE((e) => {
    if (e.Type === 'progress') {
      setRunning(jobs => jobs.map(j =>
        j.ID === e.JobID ? { ...j, Progress: e.Progress } : j,
      ))
    }
    if (e.Type === 'done' || e.Type === 'failed') {
      load()
    }
  })

  const cancel = async (id: number) => {
    try {
      await api.cancelJob(id)
      await load()
    } catch {}
  }

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-semibold text-stone-900">Queue</h1>

      {running.length > 0 && (
        <section>
          <h2 className="text-sm font-medium text-stone-500 mb-3">Running</h2>
          <div className="space-y-3">
            {running.map(job => (
              <Card key={job.ID}>
                <div className="flex items-start justify-between mb-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-stone-900 truncate">{basename(job.SourcePath)}</p>
                    <p className="text-xs text-stone-400 truncate">{job.SourcePath}</p>
                  </div>
                  <StatusBadge status={job.Status} />
                </div>
                <ProgressBar value={job.Progress} />
                <p className="text-xs text-stone-500 mt-1.5">
                  {Math.round(job.Progress * 100)}% &middot; {formatDuration(job.SourceDuration)} &middot; {formatBytes(job.SourceSize)}
                </p>
              </Card>
            ))}
          </div>
        </section>
      )}

      <section>
        <h2 className="text-sm font-medium text-stone-500 mb-3">
          Pending ({pending.length})
        </h2>
        {pending.length === 0 ? (
          <p className="text-sm text-stone-400 py-8 text-center">Queue is empty</p>
        ) : (
          <div className="space-y-2">
            {pending.map(job => (
              <div
                key={job.ID}
                className="flex items-center gap-3 bg-white border border-stone-200 rounded-lg px-4 py-3"
              >
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-stone-900 truncate">{basename(job.SourcePath)}</p>
                  <p className="text-xs text-stone-400 truncate">{job.SourcePath}</p>
                </div>
                <span className="text-xs text-stone-400 shrink-0">
                  {formatBytes(job.SourceSize)}
                </span>
                <button
                  onClick={() => cancel(job.ID)}
                  className="text-stone-400 hover:text-red-500 transition-colors shrink-0"
                  title="Cancel"
                >
                  <X size={15} />
                </button>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
