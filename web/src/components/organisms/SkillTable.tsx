import { useCallback, useMemo, useRef, useState } from 'react'
import type { Skill } from '../../api/client'
import { SkillRow, type RowFlags } from '../molecules/SkillRow'
import { EmptyState } from '../molecules/EmptyState'
import { useColumnWidths, type ColumnDef } from '../../hooks/useColumnWidths'

type Props = {
  skills: Skill[]
  agentNames: Record<string, string>
  showAgent: boolean
  /** Render the Used column (only when some skill has telemetry). */
  showUsage: boolean
  flagsFor: (id: string) => RowFlags
  busySkill: string | null
  onToggle: (skill: Skill, enabled: boolean) => void
  emptyTitle: string
  emptyHint: string
}

const WIDTHS_KEY = 'skillman.skills.columns.v1'

type Column = ColumnDef & { label: string; title?: string }

const ALL_COLUMNS: Column[] = [
  { key: 'name', label: 'Name', width: 240, min: 160 },
  { key: 'desc', label: 'Description', width: 0, min: 160, fill: true },
  { key: 'version', label: 'Version', width: 84 },
  { key: 'agent', label: 'Agent', width: 120 },
  { key: 'scope', label: 'Scope', width: 110 },
  { key: 'used', label: 'Used', width: 96, title: 'Times Claude Code invoked the skill' },
  { key: 'flags', label: 'Flags', width: 150 },
  { key: 'state', label: 'State', width: 80 },
]

export function SkillTable({ skills, agentNames, showAgent, showUsage, flagsFor, busySkill, onToggle, emptyTitle, emptyHint }: Props) {
  const tableRef = useRef<HTMLTableElement>(null)
  const columns = useMemo(
    () => ALL_COLUMNS.filter((c) => (c.key === 'agent' ? showAgent : c.key === 'used' ? showUsage : true)),
    [showAgent, showUsage],
  )
  const { widths, resizing, handleProps } = useColumnWidths(columns, WIDTHS_KEY, tableRef)
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set())
  const toggleExpanded = useCallback((id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  if (skills.length === 0) {
    return (
      <div className="table-wrap">
        <EmptyState title={emptyTitle} hint={emptyHint} />
      </div>
    )
  }
  return (
    <div className="table-wrap">
      <table ref={tableRef} className={`table table-skills${resizing ? ' is-resizing' : ''}`}>
        <colgroup>
          {columns.map((c) => (
            <col key={c.key} style={widths[c.key] !== undefined ? { width: widths[c.key] } : undefined} />
          ))}
        </colgroup>
        <thead>
          <tr>
            {columns.map((c, i) => (
              <th key={c.key} title={c.title}>
                {c.label}
                {i < columns.length - 1 && (
                  <span
                    className={`col-resizer${resizing === c.key ? ' is-active' : ''}`}
                    role="separator"
                    aria-orientation="vertical"
                    aria-label={`Resize ${c.label} column`}
                    title="Drag to resize, double-click to reset"
                    {...handleProps(c.key)}
                  />
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {skills.map((s) => (
            <SkillRow
              key={s.id}
              skill={s}
              agentName={agentNames[s.agent] ?? s.agent}
              showAgent={showAgent}
              showUsage={showUsage}
              flags={flagsFor(s.id)}
              busy={busySkill === s.id}
              expanded={expanded.has(s.id)}
              onToggleExpanded={() => toggleExpanded(s.id)}
              onToggle={(enabled) => onToggle(s, enabled)}
            />
          ))}
        </tbody>
      </table>
    </div>
  )
}
