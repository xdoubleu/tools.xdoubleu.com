import {
  collectTokens,
  elementName,
  isClassAttribute,
  parseToken,
  siblingAttr
} from './class-tokens.mjs'

const RAW_ELEMENTS = {
  h1: 'Use `PageHeader` from @/components/ui/page-header for the page title.',
  table: 'Use `Table` from @/components/ui/table (it scrolls inside its own wrapper on phones).',
  label:
    'Use `Label`/`Field` (@/components/ui/label, field) or `Checkbox`/`RadioGroupItem` `label` for a 44px hit area.'
}

const phoneBases = (attr) => collectTokens(attr.value).map((t) => parseToken(t.token))

/** Hand-rolled versions of existing primitives (docs/convention-ui-standards.md). */
export default {
  meta: {
    type: 'suggestion',
    docs: { description: 'Use components/ui primitives instead of hand-rolled equivalents' },
    messages: {
      raw: 'Raw <{{element}}>: {{hint}}',
      tablist: 'Use `SegmentedTabs` from @/components/ui/segmented-tabs for a tablist.',
      stretchedLink:
        'Stretched `absolute inset-0` link: controls on top of it are easy to miss and open the page instead. Use `LinkCard` (actions go in its non-link footer).',
      card: 'Hand-assembled card surface. Use `Card`/`SectionCard` (@/components/ui/card, section-card).',
      inset: 'Hand-assembled inset panel. Use `<Card variant="inset">`.',
      alert:
        'Hand-made banner. Use `Alert` from @/components/ui/alert (tone danger/success/warn/info).',
      badge: 'Hand-made pill. Use `Badge` from @/components/ui/badge.',
      textLink:
        'Styled text link. Use `<Button asChild variant="link">` so it gets a 44px row on phones.',
      state: 'Use `{{component}}` from @/components/ui/states for loading/error messages.',
      pageContainer:
        '`{{token}}` on PageContainer: the shell already pads pages, and width comes from `size` (`narrow`, `form`).'
    },
    schema: []
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        if (node.name.type !== 'JSXIdentifier') return
        const hint = RAW_ELEMENTS[node.name.name]
        if (hint)
          context.report({ node, messageId: 'raw', data: { element: node.name.name, hint } })

        if (node.name.name === 'p') {
          const text = node.parent.children
            .filter((c) => c.type === 'JSXText')
            .map((c) => c.value)
            .join('')
            .trim()
          if (/^Loading\b/.test(text)) {
            context.report({ node, messageId: 'state', data: { component: 'LoadingState' } })
          } else if (/^Failed to load\b/.test(text)) {
            context.report({ node, messageId: 'state', data: { component: 'ErrorState' } })
          }
        }
      },
      JSXAttribute(attr) {
        const element = elementName(attr)
        if (attr.name.name === 'role' && siblingAttr(attr, 'role') === 'tablist') {
          context.report({ node: attr, messageId: 'tablist' })
        }
        if (!isClassAttribute(attr) || attr.name.name !== 'className' || !attr.value) return
        const tokens = phoneBases(attr)
        const bases = new Set(tokens.filter((t) => t.variants.length === 0).map((t) => t.base))
        const has = (re) => [...bases].some((b) => re.test(b))

        if (element === 'PageContainer') {
          for (const b of bases) {
            if (/^(p|px|py|pt|pb|pl|pr)-/.test(b) || /^max-w-/.test(b)) {
              context.report({ node: attr, messageId: 'pageContainer', data: { token: b } })
            }
          }
          return
        }
        const isLink = element === 'Link' || element === 'a'
        if (isLink && bases.has('absolute') && bases.has('inset-0')) {
          context.report({ node: attr, messageId: 'stretchedLink' })
          return
        }
        if (
          isLink &&
          (bases.has('text-accent') ||
            tokens.some((t) => t.base === 'underline' && t.variants.includes('hover')))
        ) {
          context.report({ node: attr, messageId: 'textLink' })
          return
        }
        if (
          element === 'span' &&
          bases.has('rounded-full') &&
          bases.has('text-xs') &&
          has(/^px-/)
        ) {
          context.report({ node: attr, messageId: 'badge' })
          return
        }
        const rounded = has(/^rounded(-|$)/)
        const bordered = has(/^border$/)
        if (rounded && bordered && bases.has('bg-card')) {
          context.report({ node: attr, messageId: 'card' })
        } else if (rounded && bordered && bases.has('bg-surface')) {
          context.report({ node: attr, messageId: 'inset' })
        } else if (has(/^bg-(danger|success|warn)\/\d+$/)) {
          context.report({ node: attr, messageId: 'alert' })
        }
      }
    }
  }
}
