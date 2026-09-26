import { isClassAttribute } from './class-tokens.mjs'

/** Template-literal classNames lose to Tailwind ordering; `cn()` merges reliably. */
export default {
  meta: {
    type: 'suggestion',
    docs: { description: 'Build conditional classNames with cn(), not template literals' },
    messages: {
      template:
        'Build conditional classes with `cn()` from `@/lib/cn`, not a template literal: it resolves conflicting utilities and drops falsy parts.'
    },
    schema: []
  },
  create(context) {
    return {
      JSXAttribute(attr) {
        if (!isClassAttribute(attr) || attr.value?.type !== 'JSXExpressionContainer') return
        const expr = attr.value.expression
        if (expr.type === 'TemplateLiteral' && expr.expressions.length > 0) {
          context.report({ node: expr, messageId: 'template' })
        }
      }
    }
  }
}
