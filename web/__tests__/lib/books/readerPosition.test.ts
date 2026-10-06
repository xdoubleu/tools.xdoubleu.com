import {
  positionAt,
  rangeAtTextOffset,
  resumeFromState,
  resumeTarget,
  textOffsetAt
} from '@/lib/books/readerPosition'

function sectionDoc(html: string) {
  const doc = document.implementation.createHTMLDocument('section')
  doc.body.innerHTML = html
  return doc
}

const EPUB_SECTIONS = [{ id: 'OEBPS/cover.xhtml' }, { id: 'OEBPS/ch1.xhtml' }]
const PDF_SECTIONS = [{ id: 0 }, { id: 1 }, { id: 2 }]

describe('textOffsetAt', () => {
  it('counts the body text before a point inside a text node', () => {
    const doc = sectionDoc('<h1>Title</h1><p>One <em>two</em> three</p>')
    const em = doc.querySelector('em')!.firstChild!
    expect(textOffsetAt(em, 1)).toBe('Title'.length + 'One '.length + 1)
  })

  it('counts the text before a point between elements', () => {
    const doc = sectionDoc('<p>abc</p><p>def</p>')
    expect(textOffsetAt(doc.body, 1)).toBe(3)
  })

  it('is zero for a point outside the body', () => {
    const doc = sectionDoc('<p>abc</p>')
    expect(textOffsetAt(doc.head, 0)).toBe(0)
  })

  it('is zero for a node outside any document body', () => {
    expect(textOffsetAt(document.implementation.createHTMLDocument('x'), 0)).toBe(0)
  })
})

describe('rangeAtTextOffset', () => {
  it('lands inside the text node holding the offset', () => {
    const doc = sectionDoc('<h1>Title</h1><p>One <em>two</em> three</p>')
    const range = rangeAtTextOffset(doc, 10)
    expect(range.collapsed).toBe(true)
    expect(range.startContainer).toBe(doc.querySelector('em')!.firstChild)
    expect(range.startOffset).toBe(1)
  })

  it('round-trips with textOffsetAt', () => {
    const doc = sectionDoc('<p>alpha</p><div><p>beta <b>gamma</b></p></div>')
    for (const offset of [0, 3, 5, 8, 12, 15]) {
      const range = rangeAtTextOffset(doc, offset)
      expect(textOffsetAt(range.startContainer, range.startOffset)).toBe(offset)
    }
  })

  it('starts at the next text node on a boundary', () => {
    const doc = sectionDoc('<p>abc</p><p>def</p>')
    const range = rangeAtTextOffset(doc, 3)
    expect(range.startContainer).toBe(doc.querySelectorAll('p')[1]!.firstChild)
    expect(range.startOffset).toBe(0)
  })

  it('clamps an offset past the end to the end of the text', () => {
    const doc = sectionDoc('<p>abc</p>')
    const range = rangeAtTextOffset(doc, 99)
    expect(range.startContainer).toBe(doc.querySelector('p')!.firstChild)
    expect(range.startOffset).toBe(3)
  })

  it('falls back to the body start when the section has no text', () => {
    const doc = sectionDoc('<img alt="">')
    const range = rangeAtTextOffset(doc, 5)
    expect(range.startContainer).toBe(doc.body)
    expect(range.startOffset).toBe(0)
  })
})

describe('positionAt', () => {
  it('maps an EPUB range to the section href and its text offset', () => {
    const doc = sectionDoc('<p>Call me Ishmael.</p>')
    const range = doc.createRange()
    range.setStart(doc.querySelector('p')!.firstChild!, 8)
    expect(positionAt(EPUB_SECTIONS, 1, range)).toEqual({ href: 'OEBPS/ch1.xhtml', offset: 8 })
  })

  it('uses the section start without a range', () => {
    expect(positionAt(EPUB_SECTIONS, 0)).toEqual({ href: 'OEBPS/cover.xhtml', offset: 0 })
  })

  it('maps a PDF section to its 1-based page', () => {
    expect(positionAt(PDF_SECTIONS, 2)).toEqual({ page: 3 })
  })

  it('has no position for an unknown section', () => {
    expect(positionAt(EPUB_SECTIONS, 5)).toBeUndefined()
    expect(positionAt(undefined, 0)).toBeUndefined()
  })
})

describe('resumeTarget', () => {
  it('opens the section by href at the stored text offset', () => {
    const target = resumeTarget(EPUB_SECTIONS, {
      position: { href: 'OEBPS/ch1.xhtml', offset: 4 },
      percent: 30
    })
    expect(target).toEqual({ index: 1, anchor: expect.any(Function) })
    const doc = sectionDoc('<p>Call me Ishmael.</p>')
    const range = target && 'anchor' in target ? target.anchor?.(doc) : undefined
    expect(range?.startOffset).toBe(4)
  })

  it('opens the first section by href', () => {
    expect(
      resumeTarget(EPUB_SECTIONS, {
        position: { href: 'OEBPS/cover.xhtml', offset: 0 },
        percent: 1
      })
    ).toEqual({ index: 0, anchor: expect.any(Function) })
  })

  it('opens a PDF at the stored page', () => {
    expect(resumeTarget(PDF_SECTIONS, { position: { page: 2 }, percent: 50 })).toEqual({
      index: 1
    })
  })

  it('falls back to the percent when the position belongs to another file', () => {
    expect(resumeTarget(EPUB_SECTIONS, { position: { page: 2 }, percent: 40 })).toEqual({
      fraction: 0.4
    })
    expect(
      resumeTarget(PDF_SECTIONS, { position: { href: 'OEBPS/ch1.xhtml', offset: 0 }, percent: 10 })
    ).toEqual({ fraction: 0.1 })
    expect(
      resumeTarget(EPUB_SECTIONS, { position: { href: 'missing.xhtml', offset: 0 }, percent: 25 })
    ).toEqual({ fraction: 0.25 })
    expect(resumeTarget(PDF_SECTIONS, { position: { page: 9 }, percent: 70 })).toEqual({
      fraction: 0.7
    })
  })

  it('uses whichever form of a PDF-sourced position fits the open file', () => {
    const resume = {
      position: { page: 2 },
      alsoAt: { href: 'OEBPS/ch1.xhtml', offset: 4 },
      percent: 30
    }
    expect(resumeTarget(PDF_SECTIONS, resume)).toEqual({ index: 1 })
    const target = resumeTarget(EPUB_SECTIONS, resume)
    expect(target).toEqual({ index: 1, anchor: expect.any(Function) })
    expect(resumeTarget(EPUB_SECTIONS, { ...resume, alsoAt: { page: 9 } })).toEqual({
      fraction: 0.3
    })
  })

  it('seeks to the percent when only a percent is stored', () => {
    expect(resumeTarget(EPUB_SECTIONS, { percent: 55 })).toEqual({ fraction: 0.55 })
  })

  it('opens at the start with nothing stored', () => {
    expect(resumeTarget(EPUB_SECTIONS, { percent: 0 })).toBeNull()
    expect(resumeTarget(EPUB_SECTIONS, undefined)).toBeNull()
    expect(resumeTarget(undefined, { position: { page: 1 }, percent: 0 })).toBeNull()
  })
})

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
