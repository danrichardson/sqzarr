import { useEffect, useState } from 'react'
import { api, type Status } from '../lib/api'
import { Card } from '../components/Card'
import { Cpu, Shield, Server } from 'lucide-react'

export function Settings() {
  const [status, setStatus] = useState<Status | null>(null)

  useEffect(() => {
    api.getStatus().then(setStatus).catch(() => {})
  }, [])

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-semibold text-stone-900">Settings</h1>

      {/* Hardware */}
      <section>
        <h2 className="text-sm font-medium text-stone-500 mb-3 flex items-center gap-1.5">
          <Cpu size={14} /> Hardware
        </h2>
        <Card>
          <p className="text-sm text-stone-700">
            <span className="text-stone-400 mr-2">Active encoder</span>
            {status?.encoder ?? '—'}
          </p>
        </Card>
      </section>

      {/* Config */}
      <section>
        <h2 className="text-sm font-medium text-stone-500 mb-3 flex items-center gap-1.5">
          <Server size={14} /> Configuration
        </h2>
        <Card>
          <p className="text-sm text-stone-600">
            SQZARR is configured via <code className="text-xs bg-stone-100 px-1 py-0.5 rounded font-mono">sqzarr.toml</code>.
            Edit the config file and restart the service to change scan schedule,
            concurrency, quarantine policy, disk thresholds, and Plex settings.
          </p>
          <p className="text-sm text-stone-400 mt-2">
            Default location: <code className="text-xs bg-stone-100 px-1 py-0.5 rounded font-mono">/etc/sqzarr/sqzarr.toml</code>
          </p>
        </Card>
      </section>

      {/* Auth */}
      <section>
        <h2 className="text-sm font-medium text-stone-500 mb-3 flex items-center gap-1.5">
          <Shield size={14} /> Authentication
        </h2>
        <Card>
          <p className="text-sm text-stone-600">
            To enable password protection, generate a hash and add it to your config:
          </p>
          <pre className="mt-2 text-xs bg-stone-100 rounded p-3 font-mono overflow-x-auto text-stone-700">
            sqzarr hash-password{'\n'}
            # Paste the output into sqzarr.toml under [auth]
          </pre>
        </Card>
      </section>

      {/* About */}
      <section>
        <Card className="text-center">
          <p className="text-sm font-semibold text-stone-900">SQZARR {status?.version ?? ''}</p>
          <p className="text-xs text-stone-400 mt-1">
            Self-hosted GPU media transcoder
          </p>
          <a
            href="https://github.com/danrichardson/sqzarr"
            target="_blank"
            rel="noreferrer"
            className="text-xs text-amber-600 hover:underline mt-2 block"
          >
            github.com/danrichardson/sqzarr
          </a>
        </Card>
      </section>
    </div>
  )
}
