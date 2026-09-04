import {
  createContext,
  forwardRef,
  useContext,
  type InputHTMLAttributes,
  type HTMLAttributes
} from 'react'
import { cn } from '@/lib/cn'

interface RadioGroupContextValue {
  name: string
  value: string
  onChange: (value: string) => void
}

const RadioGroupContext = createContext<RadioGroupContextValue | null>(null)

const radioClass =
  'h-4 w-4 rounded-full border border-border bg-surface text-accent cursor-pointer transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:ring-offset-1 disabled:pointer-events-none disabled:opacity-50'

function useRadioGroup() {
  const ctx = useContext(RadioGroupContext)
  if (!ctx) throw new Error('RadioGroupItem must be inside RadioGroup')
  return ctx
}

interface RadioGroupProps extends Omit<HTMLAttributes<HTMLDivElement>, 'onChange'> {
  name: string
  value: string
  onChange: (value: string) => void
}

function RadioGroup({ name, value, onChange, className, children, ...props }: RadioGroupProps) {
  return (
    <RadioGroupContext.Provider value={{ name, value, onChange }}>
      <div role="radiogroup" className={cn('flex flex-col gap-1', className)} {...props}>
        {children}
      </div>
    </RadioGroupContext.Provider>
  )
}

interface RadioGroupItemProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'type' | 'name' | 'checked' | 'onChange'
> {
  value: string
  label: string
}

/** One labelled radio inside a `RadioGroup`. Use `Radio` for a custom layout. */
const RadioGroupItem = forwardRef<HTMLInputElement, RadioGroupItemProps>(
  ({ value, label, className, id, ...props }, ref) => {
    const ctx = useRadioGroup()
    const inputId = id ?? `${ctx.name}-${value}`
    return (
      <label
        htmlFor={inputId}
        className="inline-flex items-center gap-2 cursor-pointer select-none"
      >
        <input
          ref={ref}
          id={inputId}
          type="radio"
          name={ctx.name}
          value={value}
          checked={ctx.value === value}
          onChange={() => ctx.onChange(value)}
          className={cn(radioClass, className)}
          {...props}
        />
        <span className="text-sm">{label}</span>
      </label>
    )
  }
)

RadioGroupItem.displayName = 'RadioGroupItem'

type RadioProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>

/**
 * A bare styled radio input, for call sites that supply their own label and
 * layout (a selectable card, a visually-hidden control behind custom visuals)
 * and so can't use `RadioGroupItem`'s built-in label.
 */
const Radio = forwardRef<HTMLInputElement, RadioProps>(({ className, ...props }, ref) => (
  <input ref={ref} type="radio" className={cn(radioClass, className)} {...props} />
))

Radio.displayName = 'Radio'

export { RadioGroup, RadioGroupItem, Radio }
export type { RadioGroupProps, RadioGroupItemProps, RadioProps }
