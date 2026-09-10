type Tone = 'neutral' | 'info' | 'warn' | 'danger'

type Props = {
  tone?: Tone
  title?: string
  children: string
}

const toneClass: Record<Tone, string> = {
  neutral: '',
  info: ' is-info',
  warn: ' is-warn',
  danger: ' is-danger',
}

export function Badge({ tone = 'neutral', title, children }: Props) {
  return (
    <span className={`badge${toneClass[tone]}`} title={title}>
      {children}
    </span>
  )
}
