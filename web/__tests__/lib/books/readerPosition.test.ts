import { resumeFromState } from '@/lib/books/readerPosition'

describe('resumeFromState', () => {
  it('reads an EPUB position', () => {
    expect(
      resumeFromState({ percent: 12, position: { href: 'a.xhtml', offset: 3, page: 0 } })
    ).toEqual({ position: { href: 'a.xhtml', offset: 3 }, percent: 12 })
  })

  it('reads a PDF position', () => {
    expect(
      resumeFromState({ percent: 12, position: { href: '', offset: 0, page: 4 } })
    ).toStrictEqual({
      position: { page: 4 },
      percent: 12
    })
  })

  it('reads both forms of a PDF-sourced position', () => {
    expect(
      resumeFromState({ percent: 12, position: { href: 'a.xhtml', offset: 3, page: 4 } })
    ).toEqual({ position: { page: 4 }, alsoAt: { href: 'a.xhtml', offset: 3 }, percent: 12 })
  })

  it('reads a percent-only or missing state', () => {
    expect(resumeFromState({ percent: 40 })).toStrictEqual({ percent: 40 })
    expect(
      resumeFromState({ percent: 40, position: { href: '', offset: 0, page: 0 } })
    ).toStrictEqual({
      percent: 40
    })
    expect(resumeFromState(undefined)).toStrictEqual({ percent: 0 })
  })
})
