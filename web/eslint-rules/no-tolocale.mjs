const BANNED = new Set(['toLocaleString', 'toLocaleDateString', 'toLocaleTimeString'])

/** Dates, numbers and quantities print through lib/dates.ts and lib/format.ts so every page agrees. */
export default {
  meta: {
    type: 'suggestion',
    docs: { description: 'Format values with lib/dates and lib/format, not toLocale*' },
    messages: {
      banned:
        '`{{name}}` prints differently per browser and page. Use `@/lib/dates` (formatDate, formatDateTime, formatTime…) or `@/lib/format` (formatNumber, formatPercent, formatQuantity, pluralize).'
    },
    schema: []
  },
  create(context) {
    return {
      MemberExpression(node) {
        const name = node.property.type === 'Identifier' ? node.property.name : undefined
        if (name && BANNED.has(name))
          context.report({ node: node.property, messageId: 'banned', data: { name } })
      }
    }
  }
}
