import type { Project } from '../../api/client'
import { Button } from '../atoms/Button'
import { SearchInput } from '../atoms/SearchInput'
import { Spinner } from '../atoms/Spinner'
import { ProjectPicker } from '../molecules/ProjectPicker'
import { ScopeTabs, type ScopeTab } from '../molecules/ScopeTabs'

type Props = {
  query: string
  onQuery: (q: string) => void
  scope: ScopeTab
  onScope: (s: ScopeTab) => void
  projects: Project[]
  selectedProject: string | null
  onSelectProject: (root: string | null) => void
  onAddProject: () => void
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
      <span className="topbar-spacer" />
      <Button variant="primary" onClick={p.onScan} disabled={p.scanning} aria-busy={p.scanning}>
        {p.scanning ? <Spinner /> : null}
        {p.scanning ? 'Scanning' : 'Scan'}
      </Button>
    </header>
  )
}
