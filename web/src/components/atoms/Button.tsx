import type { ButtonHTMLAttributes, ReactNode } from 'react'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'default' | 'primary' | 'ghost'
  children: ReactNode
}

const variantClass = {
  default: '',
  primary: ' is-primary',
  ghost: ' is-ghost',
}

export function Button({ variant = 'default', className = '', children, ...rest }: Props) {
  return (
    <button type="button" className={`btn${variantClass[variant]} ${className}`.trim()} {...rest}>
      {children}
    </button>
  )
}
