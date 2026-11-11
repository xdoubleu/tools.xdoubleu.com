const TITLED = new Set(['PageHeader', 'SectionCard', 'SettingsSection'])
const HEADINGS = new Set(['SectionHeading'])

// Capitalised words that are names, not title case.
const PROPER_NOUNS = new Set([
  'Android',
  'Apple',
  'Claude',
  'Google',
  'GitHub',
  'Grafana',
  'Hardcover',
  'Kobo',
  'Netflix',
  'OAuth',
  'Open',
  'OpenCode',
  'OpenRouter',
  'PostHog',
  'Prometheus',
  'Readium',
  'Sentry',
  'Steam',
  'Todoist',
  'iCal',
  'iOS',
  'iRail'
])

/** The words after the first that start with a capital and are not names or acronyms. */
export function titleCaseWords(text) {
  return text
    .split(/\s+/)
    .slice(1)
    .filter((w) => /^[A-Z][a-z]/.test(w) && !PROPER_NOUNS.has(w.replace(/[^A-Za-z]+$/, '')))
}

const nameOf = (opening) => (opening.name.type === 'JSXIdentifier' ? opening.name.name : '')

function literalText(value) {
  if (value?.type === 'Literal' && typeof value.value === 'string') return value.value
  if (value?.type === 'JSXExpressionContainer') return literalText(value.expression)
  return undefined
}

/** Titles and section headings are sentence case (docs/convention-ui-standards.md). */
export default {
  meta: {
    type: 'suggestion',
    docs: { description: 'Write titles and section headings in sentence case' },
    messages: {
      titleCase:
        'Use sentence case: lower-case "{{words}}". Names (Kobo, Steam…) and acronyms keep their capitals; add a new name to PROPER_NOUNS in eslint-rules/sentence-case.mjs.'
    },
    schema: []
  },
  create(context) {
    const check = (node, text) => {
      const words = text === undefined ? [] : titleCaseWords(text)
      if (words.length > 0) {
        context.report({ node, messageId: 'titleCase', data: { words: words.join(' ') } })
      }
    }
    return {
      JSXOpeningElement(node) {
        if (!TITLED.has(nameOf(node))) return
        for (const attr of node.attributes) {
          if (attr.type === 'JSXAttribute' && attr.name.name === 'title') {
            check(attr, literalText(attr.value))
          }
        }
      },
      JSXElement(node) {
        if (!HEADINGS.has(nameOf(node.openingElement))) return
        for (const child of node.children) {
          if (child.type === 'JSXText') check(child, child.value.trim())
        }
      }
    }
  }
}
