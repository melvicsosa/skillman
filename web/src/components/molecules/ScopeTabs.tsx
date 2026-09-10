export type ScopeTab = 'global' | 'project'

type Props = {
  value: ScopeTab
  onChange: (v: ScopeTab) => void
}

const tabs: { id: ScopeTab; label: string }[] = [
  { id: 'global', label: 'Global' },
  { id: 'project', label: 'Projects' },
]

export function ScopeTabs({ value, onChange }: Props) {
  return (
    <div className="tabs" role="tablist" aria-label="Scope">
      {tabs.map((t) => (
        <button
          key={t.id}
          type="button"
          role="tab"
          aria-selected={value === t.id}
          className={`tab${value === t.id ? ' is-active' : ''}`}
          onClick={() => onChange(t.id)}
        >
          {t.label}
        </button>
      ))}
    </div>
  )
}
