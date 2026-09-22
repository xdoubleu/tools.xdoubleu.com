import { render, screen } from '@testing-library/react'
import ArticleReaderDialog from '@/components/ArticleReaderDialog'

function dialogContentClass(): string {
  // The dialog portal content is the element carrying the dialog className;
  // the shared scaffold passes it through DialogContent.
  const heading = screen.getByRole('heading')
  return heading.closest('[role="dialog"]')?.className ?? ''
}

describe('ArticleReaderDialog', () => {
  it('defaults to the centered card on desktop (books reader)', () => {
    render(<ArticleReaderDialog title="T" open onOpenChange={jest.fn()} />)
    const cls = dialogContentClass()
    expect(cls).toContain('max-w-2xl')
    expect(cls).toContain('lg:max-w-4xl')
    expect(cls).toContain('sm:h-[90vh]')
  })

  it('fills the whole desktop viewport when bleedDesktop is set (feeds reader)', () => {
    render(<ArticleReaderDialog title="T" open onOpenChange={jest.fn()} bleedDesktop />)
    const cls = dialogContentClass()
    expect(cls).toContain('lg:inset-0')
    expect(cls).toContain('lg:h-full')
    expect(cls).toContain('lg:w-full')
    expect(cls).toContain('lg:max-w-none')
    expect(cls).toContain('lg:rounded-none')
  })

  it('caps the prose column at a readable width inside the wide dialog', () => {
    render(
      <ArticleReaderDialog
        title="T"
        open
        onOpenChange={jest.fn()}
        html="<p>Body</p>"
        bleedDesktop
      />
    )
    const proseEl = [...document.querySelectorAll('.prose')].find((el) => el.textContent == 'Body')
    expect(proseEl?.className).toContain('lg:max-w-prose')
    expect(proseEl?.className).toContain('lg:mx-auto')
  })

  it('renders the sanitized article body', () => {
    render(<ArticleReaderDialog title="T" open onOpenChange={jest.fn()} html="<p>Body</p>" />)
    expect(screen.getByText('Body')).toBeInTheDocument()
  })
})
