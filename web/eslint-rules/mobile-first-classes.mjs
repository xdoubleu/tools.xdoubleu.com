import { parseToken, visitClassLists } from './class-tokens.mjs'

const STYLE_SIZE_PROPS = new Set(['width', 'minWidth', 'height', 'minHeight'])
const MAX_STYLE_PX = 96

/** Class shapes that break at a 375px viewport. */
export default {
  meta: {
    type: 'problem',
    docs: { description: 'Disallow classes and styles that break mobile layouts' },
    messages: {
      gridCols:
        '`{{token}}` starts at {{n}} columns on a phone. Write `grid-cols-1 sm:{{token}}` (or grid-cols-2) instead.',
      screen:
        '`{{token}}` is taller than the visible area once mobile browser chrome shows. Use the `dvh` equivalent (e.g. `min-h-dvh`).',
      vh: '`{{token}}` uses `vh`, which ignores mobile browser chrome. Use `dvh`.',
      px: '`{{token}}` is a fixed pixel size that breaks at narrow viewports. Use a responsive class or relative units.',
      style:
        'Inline `{{prop}}: {{value}}` is a fixed size wider than a phone allows. Use responsive classes instead.'
    },
    schema: []
  },
  create(context) {
    const check = (tokens) => {
      for (const { token, node } of tokens) {
        const { base, responsive } = parseToken(token)
        const grid = /^grid-cols-(\d+)$/.exec(base)
        if (grid && Number(grid[1]) >= 3 && !responsive) {
          context.report({ node, messageId: 'gridCols', data: { token: base, n: grid[1] } })
        } else if (/^(min-|max-)?h-screen$/.test(base)) {
          context.report({ node, messageId: 'screen', data: { token } })
        } else if (/\[[^\]]*(?<![a-z])\d*\.?\d+vh\b[^\]]*\]/.test(base)) {
          context.report({ node, messageId: 'vh', data: { token } })
        } else if (/^(min-|max-)?(w|h)-\[\d+(\.\d+)?px\]$/.test(base)) {
          context.report({ node, messageId: 'px', data: { token } })
        }
      }
    }
    const classVisitors = visitClassLists(check)
    return {
      ...classVisitors,
      JSXAttribute(attr) {
        classVisitors.JSXAttribute(attr)
        if (attr.name.name !== 'style' || attr.parent.name.type !== 'JSXIdentifier') return
        if (!/^[a-z]/.test(attr.parent.name.name)) return
        const expr = attr.value?.type === 'JSXExpressionContainer' ? attr.value.expression : null
        if (expr?.type !== 'ObjectExpression') return
        for (const p of expr.properties) {
          if (p.type !== 'Property' || p.key.type !== 'Identifier') continue
          if (!STYLE_SIZE_PROPS.has(p.key.name) || p.value.type !== 'Literal') continue
          const px =
            typeof p.value.value === 'number'
              ? p.value.value
              : Number(/^(\d+)px$/.exec(String(p.value.value))?.[1] ?? NaN)
          if (px > MAX_STYLE_PX) {
            context.report({
              node: p,
              messageId: 'style',
              data: { prop: p.key.name, value: String(p.value.value) }
            })
          }
        }
      }
    }
  }
}
