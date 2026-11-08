import {
  collectTokens,
  elementName,
  isClassAttribute,
  parseToken,
  siblingAttr
} from './class-tokens.mjs'

const CONTROLS = new Set(['Button', 'Input', 'Select'])
const SIZE_TOKEN = /^(text-(xs|sm)|h-(\d|\[)|py-)/

/** Controls take their height and type size from the shared scale (docs/convention-ui-standards.md). */
export default {
  meta: {
    type: 'suggestion',
    docs: { description: 'Disallow height, padding-y and text-size overrides on controls' },
    messages: {
      control:
        '`{{token}}` overrides the control height scale on <{{element}}>. Use `size` (`sm` only in dense rows and tables) instead of resizing one control.'
    },
    schema: []
  },
  create(context) {
    return {
      JSXAttribute(attr) {
        if (!isClassAttribute(attr) || attr.name.name !== 'className' || !attr.value) return
        const element = elementName(attr)
        if (!CONTROLS.has(element)) return
        if (element === 'Button' && siblingAttr(attr, 'variant') === 'link') return
        const tokens = collectTokens(attr.value)
        // A multi-line button (`h-auto`) needs its own vertical padding.
        const wraps = tokens.some((t) => parseToken(t.token).base === 'h-auto')
        for (const { token, node } of tokens) {
          const { base } = parseToken(token)
          if (SIZE_TOKEN.test(base) && !(wraps && base.startsWith('py-'))) {
            context.report({ node, messageId: 'control', data: { token, element } })
          }
        }
      }
    }
  }
}
