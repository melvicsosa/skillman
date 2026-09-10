type Props = {
  checked: boolean
  onChange: (checked: boolean) => void
  label: string
  disabled?: boolean
  busy?: boolean
}

export function Checkbox({ checked, onChange, label, disabled, busy }: Props) {
  return (
    <input
      type="checkbox"
      className={`checkbox${busy ? ' is-busy' : ''}`}
      checked={checked}
      disabled={disabled || busy}
      aria-label={label}
      title={label}
      onChange={(e) => onChange(e.target.checked)}
    />
  )
}
