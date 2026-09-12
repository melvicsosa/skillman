import { useCallback, useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent, type RefObject } from 'react'

export type ColumnDef = {
  key: string
  /** Default width in px. Ignored for the fill column. */
  width: number
  /** Minimum width in px (default 80). */
  min?: number
  /** Exactly one column absorbs whatever space the others leave. */
  fill?: boolean
}

export type ResizeHandleProps = {
  onPointerDown: (e: ReactPointerEvent<HTMLElement>) => void
  onPointerMove: (e: ReactPointerEvent<HTMLElement>) => void
  onPointerUp: (e: ReactPointerEvent<HTMLElement>) => void
  onPointerCancel: (e: ReactPointerEvent<HTMLElement>) => void
  onDoubleClick: () => void
}

const DEFAULT_MIN = 80

type Drag = {
  /** Column whose stored width changes during this drag. */
  key: string
  startX: number
  startWidth: number
  /** Widest the column may grow without squeezing the fill column below its minimum. */
  max: number
}

function readStored(storageKey: string): Record<string, number> {
  try {
    const raw = window.localStorage.getItem(storageKey)
    if (!raw) return {}
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return {}
    const out: Record<string, number> = {}
    for (const [k, v] of Object.entries(parsed as Record<string, unknown>)) {
      if (typeof v === 'number' && Number.isFinite(v)) out[k] = v
    }
    return out
  } catch {
    return {}
  }
}

function writeStored(storageKey: string, value: Record<string, number>) {
  try {
    if (Object.keys(value).length === 0) window.localStorage.removeItem(storageKey)
    else window.localStorage.setItem(storageKey, JSON.stringify(value))
  } catch {
    // Storage blocked: widths last for this page view only.
  }
}

/**
 * Resizable column widths for a fixed-layout table.
 *
 * Every column gets an explicit px width; the `fill` column takes whatever the
 * measured table width leaves over (never below its minimum). Dragging a
 * column's handle changes that column; dragging the fill column's handle
 * changes the column right after it, so the boundary follows the pointer.
 * Overrides persist in localStorage under `storageKey`.
 */
export function useColumnWidths(columns: ColumnDef[], storageKey: string, tableRef: RefObject<HTMLElement | null>) {
  const [overrides, setOverrides] = useState<Record<string, number>>(() => readStored(storageKey))
  const [tableWidth, setTableWidth] = useState(0)
  const [resizing, setResizing] = useState<string | null>(null)
  const drag = useRef<Drag | null>(null)

  useEffect(() => {
    const el = tableRef.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const measure = () => setTableWidth(el.clientWidth)
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [tableRef])

  const minOf = useCallback((c: ColumnDef) => c.min ?? DEFAULT_MIN, [])

  const widths = useMemo(() => {
    const out: Record<string, number | undefined> = {}
    let fixedSum = 0
    let fill: ColumnDef | undefined
    for (const c of columns) {
      if (c.fill) {
        fill = c
        continue
      }
      const w = Math.max(minOf(c), overrides[c.key] ?? c.width)
      out[c.key] = w
      fixedSum += w
    }
    if (fill) {
      // Before the first measurement the fill column stays auto.
      out[fill.key] = tableWidth > 0 ? Math.max(minOf(fill), tableWidth - fixedSum) : undefined
    }
    return out
  }, [columns, overrides, tableWidth, minOf])

  const persist = useCallback(
    (next: Record<string, number>) => {
      setOverrides(next)
      writeStored(storageKey, next)
    },
    [storageKey],
  )

  const reset = useCallback(
    (key: string) => {
      const next = { ...overrides }
      delete next[key]
      persist(next)
    },
    [overrides, persist],
  )

  /** Column whose width a handle on `key` edits: the next one for the fill column. */
  const targetFor = useCallback(
    (key: string): ColumnDef | undefined => {
      const i = columns.findIndex((c) => c.key === key)
      if (i < 0) return undefined
      const col = columns[i]
      return col.fill ? columns[i + 1] : col
    },
    [columns],
  )

  const onPointerDown = useCallback(
    (key: string, e: ReactPointerEvent<HTMLElement>) => {
      if (e.button !== 0) return
      const target = targetFor(key)
      if (!target || target.fill) return
      const startWidth = widths[target.key] ?? target.width
      const fill = columns.find((c) => c.fill)
      const fillWidth = fill ? (widths[fill.key] ?? minOf(fill)) : 0
      const slack = fill ? Math.max(0, fillWidth - minOf(fill)) : Number.POSITIVE_INFINITY
      drag.current = { key: target.key, startX: e.clientX, startWidth, max: startWidth + slack }
      e.currentTarget.setPointerCapture(e.pointerId)
      e.preventDefault()
      setResizing(key)
    },
    [columns, minOf, targetFor, widths],
  )

  const onPointerMove = useCallback(
    (e: ReactPointerEvent<HTMLElement>) => {
      const d = drag.current
      if (!d) return
      const col = columns.find((c) => c.key === d.key)
      if (!col) return
      const next = Math.round(Math.min(d.max, Math.max(minOf(col), d.startWidth + (e.clientX - d.startX))))
      setOverrides((prev) => (prev[d.key] === next ? prev : { ...prev, [d.key]: next }))
    },
    [columns, minOf],
  )

  const endDrag = useCallback(
    (e: ReactPointerEvent<HTMLElement>) => {
      const d = drag.current
      if (!d) return
      drag.current = null
      if (e.currentTarget.hasPointerCapture(e.pointerId)) e.currentTarget.releasePointerCapture(e.pointerId)
      setResizing(null)
      setOverrides((prev) => {
        writeStored(storageKey, prev)
        return prev
      })
    },
    [storageKey],
  )

  const handleProps = useCallback(
    (key: string): ResizeHandleProps => ({
      onPointerDown: (e) => onPointerDown(key, e),
      onPointerMove,
      onPointerUp: endDrag,
      onPointerCancel: endDrag,
      onDoubleClick: () => {
        const target = targetFor(key)
        if (target) reset(target.key)
      },
    }),
    [endDrag, onPointerDown, onPointerMove, reset, targetFor],
  )

  return { widths, resizing, handleProps }
}
