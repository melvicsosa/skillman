import type { ButtonHTMLAttributes, ReactNode, Ref } from 'react'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'default' | 'primary' | 'ghost'
  children: ReactNode
  ref?: Ref<HTMLButtonElement>
}

const variantClass = {
  default: '',
  primary: ' is-primary',
  ghost: ' is-ghost',
}

export function Button({ variant = 'default', className = '', children, type = 'button', ...rest }: Props) {
  return (
    <button type={type} className={`btn${variantClass[variant]} ${className}`.trim()} {...rest}>
      {children}
    </button>
  )
}
