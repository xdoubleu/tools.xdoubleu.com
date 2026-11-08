import { render, screen } from '@testing-library/react'
import { FieldError } from '@/components/ui/field-error'
import { Field } from '@/components/ui/field'

describe('FieldError', () => {
  it('announces the message with an id for aria-describedby', () => {
    render(<FieldError id="name-error">Required.</FieldError>)
    const error = screen.getByRole('alert')
    expect(error).toHaveTextContent('Required.')
    expect(error).toHaveAttribute('id', 'name-error')
    expect(error).toHaveClass('text-danger')
  })

  it.each([
    ['an empty string', ''],
    ['undefined', undefined],
    ['null', null],
    ['false', false]
  ])('renders nothing for %s', (_, value) => {
    const { container } = render(<FieldError id="x">{value}</FieldError>)
    expect(container).toBeEmptyDOMElement()
  })

  it('is what Field shows for its error', () => {
    render(
      <Field label="Name" htmlFor="name" error="Too long.">
        <span />
      </Field>
    )
    expect(screen.getByRole('alert')).toHaveAttribute('id', 'name-error')
  })
})
