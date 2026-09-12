import { useCallback, useEffect, useState } from 'react'

import type { Issue } from '../api/client'

export type StatusFilter = 'all' | 'actionable' | 'informational'

export type DoctorFilters = {
  status: StatusFilter
  /** Kinds the user toggled off; every other kind is shown. */
  hiddenKinds: string[]
}

export type DoctorView = {
  statusCounts: Record<StatusFilter, number>
  /** Per-kind counts after the status filter, so chips update when the status changes. */
  kindCounts: Record<string, number>
  /** Issues passing both filters, paired with their stable id. */
  visible: { issue: Issue; id: string }[]
}

const STORAGE_KEY = 'skillman.doctor.filters.v1'
const statuses: StatusFilter[] = ['all', 'actionable', 'informational']
const defaults: DoctorFilters = { status: 'actionable', hiddenKinds: [] }

/** Drift where every copy is read-only cannot be repaired; every other kind needs a human. */
export function isActionable(issue: Issue): boolean {
  if (issue.kind !== 'drift') return true
  return !(issue.copies ?? []).every((c) => c.readOnly)
}

/** Stable per-issue key: the API has no id, so derive one from the identifying fields. */
export function issueId(issue: Issue, index: number): string {
  return [issue.kind, issue.name ?? '', issue.scope ?? '', issue.path ?? '', index].join('|')
}

function passesStatus(issue: Issue, status: StatusFilter): boolean {
  if (status === 'all') return true
  return status === 'actionable' ? isActionable(issue) : !isActionable(issue)
}

export function applyDoctorFilters(issues: Issue[], filters: DoctorFilters): DoctorView {
  const statusCounts: Record<StatusFilter, number> = { all: issues.length, actionable: 0, informational: 0 }
  const kindCounts: Record<string, number> = {}
  const visible: DoctorView['visible'] = []
  const hidden = new Set(filters.hiddenKinds)
  issues.forEach((issue, index) => {
    if (isActionable(issue)) statusCounts.actionable += 1
    else statusCounts.informational += 1
    if (!passesStatus(issue, filters.status)) return
    kindCounts[issue.kind] = (kindCounts[issue.kind] ?? 0) + 1
    if (!hidden.has(issue.kind)) visible.push({ issue, id: issueId(issue, index) })
  })
  return { statusCounts, kindCounts, visible }
}

function load(): DoctorFilters {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return defaults
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return defaults
    const { status, hiddenKinds } = parsed as Partial<DoctorFilters>
    return {
      status: statuses.includes(status as StatusFilter) ? (status as StatusFilter) : defaults.status,
      hiddenKinds: Array.isArray(hiddenKinds) ? hiddenKinds.filter((k): k is string => typeof k === 'string') : [],
    }
  } catch {
    return defaults
  }
}

export type UseDoctorFilters = {
  filters: DoctorFilters
  setStatus: (status: StatusFilter) => void
  toggleKind: (kind: string) => void
  /** Re-select every kind; keeps the status filter. */
  resetKinds: () => void
  /** Back to "show everything". */
  showAll: () => void
}

/** Doctor list filters, persisted in localStorage (best effort). */
export function useDoctorFilters(): UseDoctorFilters {
  const [filters, setFilters] = useState<DoctorFilters>(load)

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(filters))
    } catch {
      // Private mode or quota: the filters still work for this session.
    }
  }, [filters])

  const setStatus = useCallback((status: StatusFilter) => setFilters((f) => ({ ...f, status })), [])
  const toggleKind = useCallback(
    (kind: string) =>
      setFilters((f) => ({
        ...f,
        hiddenKinds: f.hiddenKinds.includes(kind) ? f.hiddenKinds.filter((k) => k !== kind) : [...f.hiddenKinds, kind],
      })),
    [],
  )
  const resetKinds = useCallback(() => setFilters((f) => ({ ...f, hiddenKinds: [] })), [])
  const showAll = useCallback(() => setFilters({ status: 'all', hiddenKinds: [] }), [])

  return { filters, setStatus, toggleKind, resetKinds, showAll }
}
