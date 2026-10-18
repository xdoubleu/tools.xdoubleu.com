import { buildArxivFeed } from '@/lib/feeds/arxivQuery'

function searchQuery(url: string): string | null {
  return new URL(url).searchParams.get('search_query')
}

describe('buildArxivFeed', () => {
  it('builds a newest-first query for one category', () => {
    const feed = buildArxivFeed({ categories: ' cs.AI ', keywords: '', author: '' })
    expect(feed).not.toBeNull()
    const url = new URL(feed!.url)
    expect(url.origin + url.pathname).toBe('https://export.arxiv.org/api/query')
    expect(url.searchParams.get('search_query')).toBe('cat:cs.AI')
    expect(url.searchParams.get('sortBy')).toBe('submittedDate')
    expect(url.searchParams.get('sortOrder')).toBe('descending')
    expect(url.searchParams.get('max_results')).toBe('20')
    expect(feed!.title).toBe('arXiv: cs.AI')
  })

  it('ORs categories and ANDs keywords and the author', () => {
    const feed = buildArxivFeed({
      categories: 'cs.AI, cs.LG',
      keywords: 'agents,  tool   use , "planning"',
      author: 'Ada Lovelace'
    })
    expect(searchQuery(feed!.url)).toBe(
      '(cat:cs.AI OR cat:cs.LG) AND (ti:"agents" OR abs:"agents") AND ' +
        '(ti:"tool use" OR abs:"tool use") AND (ti:"planning" OR abs:"planning") AND ' +
        'au:"Ada Lovelace"'
    )
    expect(feed!.title).toBe('arXiv: agents, tool use, planning, Ada Lovelace · cs.AI, cs.LG')
  })

  it('strips query syntax so a term cannot change the query', () => {
    const feed = buildArxivFeed({
      categories: 'cs.AI',
      keywords: 'abs:foo, GPT), ANDNOT',
      author: ''
    })
    expect(searchQuery(feed!.url)).toBe(
      'cat:cs.AI AND (ti:"abs foo" OR abs:"abs foo") AND (ti:"GPT" OR abs:"GPT") AND ' +
        '(ti:"ANDNOT" OR abs:"ANDNOT")'
    )
  })

  it('accepts a category without a subject class', () => {
    const feed = buildArxivFeed({ categories: 'hep-th', keywords: '', author: '' })
    expect(searchQuery(feed!.url)).toBe('cat:hep-th')
  })

  it('treats a comma in the author as a space', () => {
    const feed = buildArxivFeed({ categories: 'cs.AI', keywords: '', author: 'Lovelace,Ada' })
    expect(searchQuery(feed!.url)).toBe('cat:cs.AI AND au:"Lovelace Ada"')
  })

  it('returns null when any category is invalid', () => {
    expect(buildArxivFeed({ categories: 'cs.AI, bad!', keywords: '', author: '' })).toBeNull()
  })

  it('returns null without a category', () => {
    expect(buildArxivFeed({ categories: ' , ', keywords: 'agents', author: '' })).toBeNull()
  })

  it('returns null for a category that is not an arXiv category', () => {
    expect(buildArxivFeed({ categories: 'cs.AI OR all:x', keywords: '', author: '' })).toBeNull()
  })
})
