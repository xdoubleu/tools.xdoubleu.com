import {
  collectTokens,
  elementName,
  isClassAttribute,
  parseToken,
  siblingAttr
} from './class-tokens.mjs'

const TAPPABLE = new Set([
  'Button',
  'TogglePill',
  'ToggleIconButton',
  'MenuItem',
  'PopoverTrigger',
  'DialogClose'
])
const FIELDS = new Set(['Input', 'Select', 'Textarea', 'Combobox', 'DateInput'])
const TITLE_OK = new Set(['iframe', 'abbr', 'link', 'style', 'svg'])
const MIN_TAP = 11 // Tailwind units: h-11 = 44px
const MIN_FIXED_FIELD_WIDTH = 20

const size = (base, prefix) => {
  const m = new RegExp(`^${prefix}-(\\d+(\\.5)?)$`).exec(base)
  return m ? Number(m[1]) : null
}

/** Controls stay ≥44px and fields stay ≥16px on phones (docs/convention-ui-standards.md). */
export default {
  meta: {
    type: 'problem',
    docs: { description: 'Keep tap targets and form fields usable on phones' },
    messages: {
      small:
        '`{{token}}` shrinks <{{element}}> below 44px on phones. Drop it (the primitive is 44px below `sm`) or prefix it with `sm:`.',
      auto: '`h-auto` lets <{{element}}> shrink to its text height on phones. Add `min-h-11` or use a sized variant.',
      fieldText:
        '`{{token}}` on <{{element}}> is under 16px, so iOS zooms on focus. The primitive already uses `text-base md:text-sm`.',
      fieldWidth:
        '`{{token}}` fixes <{{element}}> width on phones, overflowing narrow cards. Use `min-w-0 flex-1` or `w-full sm:{{token}}`.',
      inputMode:
        '`type="number"` needs `inputMode` (`numeric`/`decimal`) so phones show the number keypad.',
      title:
        '`title` tooltips never show on touch. Put the text in the UI (or `aria-label` for an icon button).'
    },
    schema: []
  },
  create(context) {
    return {
      JSXAttribute(attr) {
        const element = elementName(attr)
        const name = attr.name.name

        if (name === 'title' && attr.value) {
          const lower = /^[a-z]/.test(element)
          if (
            (lower && !TITLE_OK.has(element)) ||
            ['TableCell', 'Button', 'Link'].includes(element)
          ) {
            context.report({ node: attr, messageId: 'title' })
          }
        }

        if (name === 'type' && element === 'Input' && siblingAttr(attr, 'type') === 'number') {
          if (siblingAttr(attr, 'inputMode') === undefined) {
            context.report({ node: attr, messageId: 'inputMode' })
          }
        }

        if (!isClassAttribute(attr) || name !== 'className' || !attr.value) return
        const isTappable = TAPPABLE.has(element)
        const isField = FIELDS.has(element)
        if (!isTappable && !isField) return

        const tokens = collectTokens(attr.value).map((t) => ({ ...t, ...parseToken(t.token) }))
        const phone = tokens.filter((t) => !t.responsive && t.variants.length === 0)
        const hasMinH = phone.some((t) => (size(t.base, 'min-h') ?? 0) >= MIN_TAP)
        const isLink = element === 'Button' && siblingAttr(attr, 'variant') === 'link'

        for (const t of phone) {
          const h = size(t.base, 'h') ?? size(t.base, 'size')
          if (h !== null && h < MIN_TAP && !hasMinH) {
            context.report({ node: t.node, messageId: 'small', data: { token: t.base, element } })
          } else if (t.base === 'h-auto' && !hasMinH && !isLink && !isField) {
            context.report({ node: t.node, messageId: 'auto', data: { element } })
          }
          if (isField && /^text-(xs|sm)$/.test(t.base)) {
            context.report({
              node: t.node,
              messageId: 'fieldText',
              data: { token: t.base, element }
            })
          }
          const w = size(t.base, 'w')
          if (isField && w !== null && w >= MIN_FIXED_FIELD_WIDTH) {
            context.report({
              node: t.node,
              messageId: 'fieldWidth',
              data: { token: t.base, element }
            })
          }
        }
      }
    }
  }
}
