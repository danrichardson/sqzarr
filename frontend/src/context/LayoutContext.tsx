import { createContext, useContext, useEffect, useState, useCallback, type ReactNode } from 'react'
import { api } from '../lib/api'

interface LayoutContextValue {
  originalsCount: number
  refreshOriginals: () => void
}

const LayoutContext = createContext<LayoutContextValue | null>(null)

export function LayoutProvider({ children }: { children: ReactNode }) {
  const [originalsCount, setOriginalsCount] = useState(0)

  const refreshOriginals = useCallback(() => {
    api.listOriginals()
      .then(r => setOriginalsCount(r?.length ?? 0))
      .catch(() => {})
  }, [])

  useEffect(() => { refreshOriginals() }, [refreshOriginals])

  return (
    <LayoutContext.Provider value={{ originalsCount, refreshOriginals }}>
      {children}
    </LayoutContext.Provider>
  )
}

export function useLayoutContext() {
  const ctx = useContext(LayoutContext)
  if (!ctx) throw new Error('useLayoutContext must be used within LayoutProvider')
  return ctx
}
