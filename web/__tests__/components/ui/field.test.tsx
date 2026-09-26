import { render, screen } from '@testing-library/react'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

describe('Field', () => {
  it('labels its control', () => {
    render(
      <Field label="Name" htmlFor="name" hint="Shown to family">
        <Input id="name" />
      </Field>
    )
    expect(screen.getByLabelText('Name')).toBeInTheDocument()
    expect(screen.getByText('Shown to family')).toBeInTheDocument()
  })

  it('shows the error instead of the hint', () => {
    render(
      <Field label="Name" htmlFor="name" hint="Shown to family" error="Required">
        <Input id="name" />
      </Field>
    )
    expect(screen.getByRole('alert')).toHaveTextContent('Required')
    expect(screen.queryByText('Shown to family')).not.toBeInTheDocument()
  })
})
