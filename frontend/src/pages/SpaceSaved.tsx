import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type SavingsEntry } from '../lib/api'
import { Card, CardTitle } from '../components/Card'
import { formatBytes, basename, timeAgo } from '../lib/utils'
import { ArrowLeft } from 'lucide-react'

export function SpaceSaved() {
  const [entries, setEntries] = useState<SavingsEntry[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.listSavings()
      .then(setEntries)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const totalSaved = entries.reduce((sum, e) => sum + e.bytes_saved, 0)

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <Link
          to="/"
          className="text-stone-400 hover:text-stone-600 transition-colors"
        >
          <ArrowLeft size={20} />
        </Link>
        <h1 className="text-xl font-semibold text-stone-900">Space Saved Breakdown</h1>
      </div>

      {/* Summary */}
      <Card>
        <CardTitle>Total Space Saved</CardTitle>
        <p className="text-2xl font-semibold text-stone-900">{formatBytes(totalSaved)}</p>
        <p className="text-sm text-stone-500 mt-1">{entries.length} file{entries.length !== 1 ? 's' : ''}</p>
      </Card>

      {/* Table */}
      {loading ? (
        <p className="text-sm text-stone-400">Loading...</p>
      ) : entries.length === 0 ? (
        <p className="text-sm text-stone-500">No savings data yet.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-stone-200 text-left text-xs font-medium text-stone-500">
                <th className="pb-2 pr-4">File</th>
                <th className="pb-2 pr-4 text-right">Original</th>
                <th className="pb-2 pr-4 text-right">New Size</th>
                <th className="pb-2 pr-4 text-right">Saved</th>
                <th className="pb-2 pr-4 text-right">%</th>
                <th className="pb-2 text-right">Finished</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e) => {
                const pct = e.source_size > 0 ? ((e.bytes_saved / e.source_size) * 100) : 0
                const finished = e.finished_at?.Valid ? e.finished_at.Time : null
                return (
                  <tr key={e.id} className="border-b border-stone-100 hover:bg-stone-50">
                    <td className="py-2 pr-4">
                      <p className="font-medium text-stone-900 truncate max-w-xs" title={e.source_path}>
                        {basename(e.source_path)}
                      </p>
                      <p className="text-xs text-stone-400 truncate max-w-xs">{e.source_path}</p>
                    </td>
                    <td className="py-2 pr-4 text-right text-stone-700 whitespace-nowrap">{formatBytes(e.source_size)}</td>
                    <td className="py-2 pr-4 text-right text-stone-700 whitespace-nowrap">{formatBytes(e.output_size)}</td>
                    <td className="py-2 pr-4 text-right font-medium text-green-700 whitespace-nowrap">{formatBytes(e.bytes_saved)}</td>
                    <td className="py-2 pr-4 text-right text-stone-500 whitespace-nowrap">{pct.toFixed(1)}%</td>
                    <td className="py-2 text-right text-stone-500 whitespace-nowrap">{finished ? timeAgo(finished) : '—'}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
