import { useFactoryModels } from '@/hooks/useFactory'
import { Combobox } from '@/components/ui/combobox'
import { Field } from '@/components/ui/field'

/** The OpenCode model input with the catalog's models as searchable suggestions.
 * Every model OpenRouter serves is a text chat model OpenCode can run, so the
 * catalog is the picker's whole list; any id not in it can still be typed. */
export function FactoryModelPicker({
  id,
  label,
  hint,
  value,
  onChange,
  required
}: {
  id: string
  label: string
  hint: string
  value: string
  onChange: (value: string) => void
  /** Marks the field required; a blank OpenCode model is refused server-side. */
  required?: boolean
}) {
  const { data: models } = useFactoryModels()
  return (
    <Field label={label} htmlFor={id} hint={hint}>
      <Combobox
        id={id}
        value={value}
        suggestions={models ?? []}
        onChange={onChange}
        onSelect={onChange}
        inputProps={{
          required,
          autoCapitalize: 'none',
          autoCorrect: 'off'
        }}
      />
    </Field>
  )
}
