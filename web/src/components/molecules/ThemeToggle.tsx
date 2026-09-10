import { useTheme } from '../../hooks/useTheme'
import { Icon } from '../atoms/Icon'

/** Sun/moon icon button that flips between the light and dark palettes. */
export function ThemeToggle() {
  const [theme, toggle] = useTheme()
  const label = theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'
  return (
    <button type="button" className="btn is-icon" onClick={toggle} aria-label={label} title={label}>
      <Icon name={theme === 'dark' ? 'sun' : 'moon'} />
    </button>
  )
}
