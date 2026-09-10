import type { Project } from '../../api/client'
import { Button } from '../atoms/Button'
import { SearchInput } from '../atoms/SearchInput'
import { Spinner } from '../atoms/Spinner'
import { ProjectPicker } from '../molecules/ProjectPicker'
import { ScopeTabs, type ScopeTab } from '../molecules/ScopeTabs'

export type SkillSort = 'name' | 'used'

type Props = {
  query: string
  onQuery: (q: string) => void
  scope: ScopeTab
  onScope: (s: ScopeTab) => void
  projects: Project[]
  selectedProject: string | null
  onSelectProject: (root: string | null) => void
  onAddProject: () => void
  sort: SkillSort
  onSort: (s: SkillSort) => void
  /** Hide the sort control when no skill carries usage telemetry. */
  sortable: boolean
  scanning: boolean
  onScan: () => void
}

export function TopBar(p: Props) {
  return (
    <header className="topbar">
      <SearchInput value={p.query} onChange={p.onQuery} />
      <ScopeTabs value={p.scope} onChange={p.onScope} />
      {p.scope === 'project' && (
        <ProjectPicker
          projects={p.projects}
          selected={p.selectedProject}
          onSelect={p.onSelectProject}
          onAdd={p.onAddProject}
        />
      )}
      {p.sortable && (
        <select className="select" aria-label="Sort" value={p.sort} onChange={(e) => p.onSort(e.target.value as SkillSort)}>
          <option value="name">Sort: name</option>
          <option value="used">Sort: most used</option>
        </select>
      )}
      <span className="topbar-spacer" />
      <Button variant="primary" onClick={p.onScan} disabled={p.scanning} aria-busy={p.scanning}>
        {p.scanning ? <Spinner /> : null}
        {p.scanning ? 'Scanning' : 'Scan'}
      </Button>
    </header>
  )
}
