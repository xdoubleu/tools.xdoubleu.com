import { collectTokens, elementName, isClassAttribute, parseToken } from './class-tokens.mjs'

const PADDING = /^p[xytblr]?-/
const RADIUS = /^rounded(-|$)/

/** Card spacing and corners come from its props (docs/convention-ui-standards.md). */
export default {
  meta: {
    type: 'suggestion',
    docs: { description: 'Disallow padding and radius overrides on Card' },
    messages: {
      padding:
        '`{{token}}` pads <Card> by hand. Use `padding` (`none`, `sm`, `md`); put inner spacing on the content.',
      radius: '`{{token}}` changes <Card> corners. Cards keep the `rounded-2xl` token.'
    },
    schema: []
  },
  create(context) {
    return {
      JSXAttribute(attr) {
        if (!isClassAttribute(attr) || attr.name.name !== 'className' || !attr.value) return
        if (elementName(attr) !== 'Card') return
        for (const { token, node } of collectTokens(attr.value)) {
          const { base } = parseToken(token)
          if (PADDING.test(base)) {
            context.report({ node, messageId: 'padding', data: { token } })
          } else if (RADIUS.test(base)) {
            context.report({ node, messageId: 'radius', data: { token } })
          }
        }
      }
    }
  }
}
