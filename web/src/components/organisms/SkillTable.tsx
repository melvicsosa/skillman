import type { Skill } from '../../api/client'
import { SkillRow, type RowFlags } from '../molecules/SkillRow'

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

export function SkillTable({ skills, agentNames, showAgent, showUsage, flagsFor, busySkill, onToggle, emptyTitle, emptyHint }: Props) {
  if (skills.length === 0) {
    return (
      <div className="table-wrap">
        <div className="state-box">
          <h3>{emptyTitle}</h3>
          <p>{emptyHint}</p>
        </div>
      </div>
    )
  }
  return (
    <div className="table-wrap">
      <table className="table">
        <colgroup>
          <col className="col-name" />
          <col />
          <col className="col-version" />
          {showAgent && <col className="col-agent" />}
          <col className="col-scope" />
          {showUsage && <col className="col-used" />}
          <col className="col-flags" />
          <col className="col-state" />
        </colgroup>
        <thead>
          <tr>
            <th>Name</th>
            <th>Description</th>
            <th>Version</th>
            {showAgent && <th>Agent</th>}
            <th>Scope</th>
            {showUsage && <th title="Times Claude Code invoked the skill">Used</th>}
            <th>Flags</th>
            <th>State</th>
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
              onToggle={(enabled) => onToggle(s, enabled)}
            />
          ))}
        </tbody>
      </table>
    </div>
  )
}
