// Builds an arXiv API query feed (newest submissions first) from
// comma-separated categories, keywords and an author.

export interface ArxivQueryInput {
  categories: string
  keywords: string
  author: string
}

export interface ArxivFeed {
  url: string
  title: string
}

// The API returns at most this many entries, matching the per-poll ingest cap.
const MAX_RESULTS = '20'

const CATEGORY = /^[a-z-]+(\.[a-z-]+)?$/i

// Drops query syntax (quotes, parentheses, field prefixes) from each term.
function splitList(raw: string): string[] {
  return raw
    .split(',')
    .map((t) =>
      t
        .replace(/["():]/g, ' ')
        .trim()
        .replace(/\s+/g, ' ')
    )
    .filter(Boolean)
}

// Quoting keeps terms such as AND or ANDNOT from acting as operators.
function quoted(term: string): string {
  return `"${term}"`
}

// Returns null when no category is given or one isn't a valid arXiv
// category (e.g. cs.AI, math.CO, hep-th).
export function buildArxivFeed(input: ArxivQueryInput): ArxivFeed | null {
  const categories = splitList(input.categories)
  if (categories.length === 0 || !categories.every((c) => CATEGORY.test(c))) {
    return null
  }
  const keywords = splitList(input.keywords)
  const author = splitList(input.author.replace(/,/g, ' '))[0] ?? ''

  const catClause = categories.map((c) => `cat:${c}`).join(' OR ')
  const clauses = [categories.length > 1 ? `(${catClause})` : catClause]
  for (const k of keywords) {
    clauses.push(`(ti:${quoted(k)} OR abs:${quoted(k)})`)
  }
  if (author) clauses.push(`au:${quoted(author)}`)

  const params = new URLSearchParams({
    search_query: clauses.join(' AND '),
    sortBy: 'submittedDate',
    sortOrder: 'descending',
    max_results: MAX_RESULTS
  })
  const topics = [...keywords, ...(author ? [author] : [])]
  const prefix = topics.length > 0 ? `${topics.join(', ')} · ` : ''
  return {
    url: `https://export.arxiv.org/api/query?${params.toString()}`,
    title: `arXiv: ${prefix}${categories.join(', ')}`
  }
}
