import type { Project } from '../../api/client'
import { Button } from '../atoms/Button'
import { SearchInput } from '../atoms/SearchInput'
import { Spinner } from '../atoms/Spinner'
import { ProjectManager } from '../molecules/ProjectManager'
import { ProjectPicker } from '../molecules/ProjectPicker'
import { ScopeTabs, type ScopeTab } from '../molecules/ScopeTabs'
import { Select } from '../molecules/Select'
import { ThemeToggle } from '../molecules/ThemeToggle'

export type SkillSort = 'name' | 'used'

type Props = {
  query: string
  onQuery: (q: string) => void
  scope: ScopeTab
  onScope: (s: ScopeTab) => void
  projects: Project[]
  selectedProject: string | null
  onSelectProject: (root: string | null) => void
  onAddProject: (root: string, opts?: { createSkillsDir?: boolean }) => Promise<void>
  onRemoveProject: (project: Project) => Promise<void>
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
        <>
          <ProjectPicker projects={p.projects} selected={p.selectedProject} onSelect={p.onSelectProject} />
          <ProjectManager projects={p.projects} onAdd={p.onAddProject} onRemove={p.onRemoveProject} />
        </>
      )}
      {p.sortable && (
        <Select<SkillSort>
          ariaLabel="Sort"
          value={p.sort}
          onChange={p.onSort}
          options={[
            { value: 'name', label: 'Sort: name' },
            { value: 'used', label: 'Sort: most used' },
          ]}
        />
      )}
      <span className="topbar-spacer" />
      <ThemeToggle />
      <Button variant="primary" onClick={p.onScan} disabled={p.scanning} aria-busy={p.scanning}>
        {p.scanning ? <Spinner /> : null}
        {p.scanning ? 'Scanning' : 'Scan'}
      </Button>
    </header>
  )
}
