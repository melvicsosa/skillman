import type { Project } from '../../api/client'
import { Select } from './Select'

type Props = {
  projects: Project[]
  /** Selected project root, or null for all projects. */
  selected: string | null
  onSelect: (root: string | null) => void
}

/** Project filter: "All projects" plus every registered project (by root). */
export function ProjectPicker({ projects, selected, onSelect }: Props) {
  const options = [
    { value: '', label: `All projects${projects.length ? ` (${projects.length})` : ''}` },
    ...projects.map((p) => ({ value: p.root, label: p.name, hint: p.root, title: p.root })),
  ]
  return (
    <Select
      ariaLabel="Project"
      className="is-project"
      value={selected ?? ''}
      options={options}
      onChange={(v) => onSelect(v === '' ? null : v)}
    />
  )
}
