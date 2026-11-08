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

/** Hand-built versions of SectionHeading, FieldError, ProgressBar and RemoveButton (docs/convention-ui-standards.md). */
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
      remove: 'Hand-built × button. Use `RemoveButton` from @/components/ui/remove-button.'
    },
    schema: []
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        if (node.name.type === 'JSXIdentifier' && node.name.name === 'Button') {
          if (/^[×✕✖]$/.test(ownText(node))) context.report({ node, messageId: 'remove' })
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
