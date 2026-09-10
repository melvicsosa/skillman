import { useCallback, useEffect, useState } from 'react'

export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'skillman.theme'

function stored(): Theme | null {
  try {
    const v = window.localStorage.getItem(STORAGE_KEY)
    return v === 'light' || v === 'dark' ? v : null
  } catch {
    return null
  }
}

function systemTheme(): Theme {
  return window.matchMedia?.('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

/** Current theme: explicit choice from localStorage, else prefers-color-scheme.
 * index.html sets data-theme before React loads, so this only keeps it in sync. */
export function useTheme(): [Theme, () => void] {
  const [explicit, setExplicit] = useState<Theme | null>(stored)
  const [system, setSystem] = useState<Theme>(systemTheme)
  const theme = explicit ?? system

  useEffect(() => {
    const mq = window.matchMedia?.('(prefers-color-scheme: light)')
    if (!mq) return
    const onChange = () => setSystem(mq.matches ? 'light' : 'dark')
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])

  useEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])

  const toggle = useCallback(() => {
    const next: Theme = theme === 'dark' ? 'light' : 'dark'
    setExplicit(next)
    try {
      window.localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // Storage blocked: the choice lasts for this page view only.
    }
  }, [theme])

  return [theme, toggle]
}
