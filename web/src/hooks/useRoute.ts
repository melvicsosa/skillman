import { useCallback, useEffect, useState } from 'react'

import { ALL_AGENTS, VIEW_DISCOVER, VIEW_SETTINGS, VIEW_VAULT } from '../components/organisms/Sidebar'

const viewPaths: Record<string, string> = {
  [VIEW_VAULT]: '/vault',
  [VIEW_DISCOVER]: '/discover',
  [VIEW_SETTINGS]: '/settings',
}

/** Maps a URL path to a sidebar view id. Unknown paths fall back to ALL_AGENTS. */
export function viewFromPath(pathname: string): string {
  const path = pathname.replace(/\/+$/, '') || '/'
  if (path === '/') return ALL_AGENTS
  for (const [view, p] of Object.entries(viewPaths)) if (p === path) return view
  const m = /^\/agent\/([^/]+)$/.exec(path)
  if (m) {
    try {
      return decodeURIComponent(m[1])
    } catch {
      return ALL_AGENTS
    }
  }
  return ALL_AGENTS
}

/** Inverse of viewFromPath: any id that is not a library view is an agent. */
export function pathFromView(view: string): string {
  if (view === ALL_AGENTS) return '/'
  return viewPaths[view] ?? `/agent/${encodeURIComponent(view)}`
}

/** Sidebar selection backed by the History API; no router library. */
export function useRoute(): [string, (view: string) => void] {
  const [selected, setSelected] = useState<string>(() => viewFromPath(window.location.pathname))

  useEffect(() => {
    const onPop = () => setSelected(viewFromPath(window.location.pathname))
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  const select = useCallback((view: string) => {
    const path = pathFromView(view)
    if (window.location.pathname !== path) window.history.pushState(null, '', path)
    setSelected(view)
  }, [])

  return [selected, select]
}
