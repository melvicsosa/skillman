type Props = {
  on: boolean
  onChange: (on: boolean) => void
  disabled?: boolean
  busy?: boolean
  label: string
}

export function Toggle({ on, onChange, disabled, busy, label }: Props) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      aria-label={label}
      title={label}
      className={`toggle${on ? ' is-on' : ''}${busy ? ' is-busy' : ''}`}
      disabled={disabled || busy}
      onClick={(e) => {
        e.stopPropagation()
        onChange(!on)
      }}
    />
  )
}
