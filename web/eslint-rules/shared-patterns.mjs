import {
  collectTokens,
  elementName,
  isClassAttribute,
  parseToken,
  siblingAttr
} from './class-tokens.mjs'

const ownText = (opening) =>
  opening.parent.children
    .filter((c) => c.type === 'JSXText')
    .map((c) => c.value)
    .join('')
    .trim()

/** Hand-built versions of SectionHeading, FieldError, ProgressBar, RemoveButton and SegmentedTabs (docs/convention-ui-standards.md). */
export default {
  meta: {
    type: 'suggestion',
    docs: {
      description:
        'Use SectionHeading, FieldError, ProgressBar and RemoveButton over hand-built markup'
    },
    messages: {
      heading:
        'Hand-styled <{{element}}>. Use `SectionHeading` (@/components/ui/section-heading), `level="eyebrow"` for the small uppercase label.',
      fieldError:
        'Hand-styled error line. Use `FieldError` (@/components/ui/field-error) and point the control’s `aria-describedby` at its `id`.',
      progress: 'Hand-built progress bar. Use `ProgressBar` from @/components/ui/progress-bar.',
      remove: 'Hand-built × button. Use `RemoveButton` from @/components/ui/remove-button.',
      tabs: 'A `{{element}}` with role="tab" is a hand-built tab bar. Use `SegmentedTabs` from @/components/ui/segmented-tabs for switching views, sections or scope.'
    },
    schema: []
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        if (node.name.type === 'JSXIdentifier' && node.name.name === 'Button') {
          if (/^[×✕✖]$/.test(ownText(node))) context.report({ node, messageId: 'remove' })
        }
        if (
          node.name.type === 'JSXIdentifier' &&
          ['Button', 'TogglePill'].includes(node.name.name)
        ) {
          const role = node.attributes.find(
            (a) => a.type === 'JSXAttribute' && a.name.name === 'role'
          )
          if (role?.value?.type === 'Literal' && role.value.value === 'tab') {
            context.report({ node, messageId: 'tabs', data: { element: node.name.name } })
          }
        }
      },
      JSXAttribute(attr) {
        const element = elementName(attr)
        if (!/^[a-z]/.test(element)) return
        if (attr.name.name === 'role' && siblingAttr(attr, 'role') === 'progressbar') {
          context.report({ node: attr, messageId: 'progress' })
          return
        }
        if (!isClassAttribute(attr) || attr.name.name !== 'className' || !attr.value) return
        const bases = new Set(
          collectTokens(attr.value)
            .map((t) => parseToken(t.token))
            .filter((t) => t.variants.length === 0)
            .map((t) => t.base)
        )
        if (element === 'h2' || (element === 'h3' && bases.has('uppercase'))) {
          context.report({ node: attr, messageId: 'heading', data: { element } })
        } else if (
          element === 'p' &&
          bases.has('text-danger') &&
          (bases.has('text-xs') || bases.has('text-sm')) &&
          siblingAttr(attr, 'role') === 'alert'
        ) {
          context.report({ node: attr, messageId: 'fieldError' })
        }
      }
    }
  }
}
