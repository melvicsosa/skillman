import type { Project } from '../../api/client'

const ADD = '__add__'

type Props = {
  projects: Project[]
  selected: string | null
  onSelect: (root: string | null) => void
  onAdd: () => void
}

export function ProjectPicker({ projects, selected, onSelect, onAdd }: Props) {
  return (
    <select
      className="select"
      aria-label="Project"
      value={selected ?? ''}
      onChange={(e) => {
        const v = e.target.value
        if (v === ADD) {
          onAdd()
          return
        }
        onSelect(v === '' ? null : v)
      }}
    >
      <option value="">All projects{projects.length ? ` (${projects.length})` : ''}</option>
      {projects.map((p) => (
        <option key={p.id} value={p.root} title={p.root}>
          {p.name} — {p.root}
        </option>
      ))}
      <option value={ADD}>Add project…</option>
    </select>
  )
}
