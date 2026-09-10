import type { ReactNode } from 'react'

export type IconName = 'vault' | 'discover' | 'settings' | 'sun' | 'moon' | 'check' | 'chevron' | 'plus' | 'close' | 'folder'

const paths: Record<IconName, ReactNode> = {
  vault: (
    <>
      <rect x="2" y="3" width="12" height="10" rx="1.5" />
      <circle cx="8" cy="8" r="2.25" />
      <path d="M8 5.75v-1M8 11.25v-1M4 13v1M12 13v1" />
    </>
  ),
  discover: (
    <>
      <circle cx="8" cy="8" r="6" />
      <path d="m10.5 5.5-1.5 3.5-3.5 1.5 1.5-3.5z" />
    </>
  ),
  settings: (
    <>
      <circle cx="8" cy="8" r="2" />
      <path d="M8 1.75v1.5M8 12.75v1.5M1.75 8h1.5M12.75 8h1.5M3.58 3.58l1.06 1.06M11.36 11.36l1.06 1.06M3.58 12.42l1.06-1.06M11.36 4.64l1.06-1.06" />
      <circle cx="8" cy="8" r="4.25" />
    </>
  ),
  sun: (
    <>
      <circle cx="8" cy="8" r="3" />
      <path d="M8 1.5v1.25M8 13.25v1.25M1.5 8h1.25M13.25 8h1.25M3.4 3.4l.9.9M11.7 11.7l.9.9M3.4 12.6l.9-.9M11.7 4.3l.9-.9" />
    </>
  ),
  moon: <path d="M13.5 9.5A5.5 5.5 0 0 1 6.5 2.5a5.5 5.5 0 1 0 7 7z" />,
  check: <path d="m3.5 8.5 3 3 6-7" />,
  chevron: <path d="m4.5 6.5 3.5 3.5 3.5-3.5" />,
  plus: <path d="M8 3.5v9M3.5 8h9" />,
  close: <path d="m4.5 4.5 7 7M11.5 4.5l-7 7" />,
  folder: <path d="M2 4.5A1.5 1.5 0 0 1 3.5 3h2.6l1.4 1.5h5A1.5 1.5 0 0 1 14 6v5.5a1.5 1.5 0 0 1-1.5 1.5h-9A1.5 1.5 0 0 1 2 11.5z" />,
}

type Props = {
  name: IconName
  size?: number
  className?: string
}

/** Inline 16px stroke icon; inherits color via currentColor. */
export function Icon({ name, size = 16, className = '' }: Props) {
  return (
    <svg
      className={`icon ${className}`.trim()}
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {paths[name]}
    </svg>
  )
}
