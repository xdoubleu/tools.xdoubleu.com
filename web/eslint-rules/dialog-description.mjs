const nameOf = (opening) => (opening.name.type === 'JSXIdentifier' ? opening.name.name : '')

// A description may sit inside a `{cond && …}` or a fragment and still be a child of the header.
const TRANSPARENT = new Set([
  'JSXExpressionContainer',
  'LogicalExpression',
  'ConditionalExpression',
  'JSXFragment'
])

function enclosingHeader(element) {
  let p = element.parent
  while (p && TRANSPARENT.has(p.type)) p = p.parent
  return p?.type === 'JSXElement' && nameOf(p.openingElement) === 'DialogHeader'
}

/** `DialogHeader` is a title-and-close row; a description belongs below it (docs/convention-ui-standards.md). */
export default {
  meta: {
    type: 'problem',
    docs: { description: 'Keep DialogDescription out of DialogHeader’s direct children' },
    messages: {
      inHeader:
        '`DialogDescription` is a direct child of `DialogHeader`, a flex row, so it lands beside the title and squeezes it. Put it after the header, or wrap the title and description in a `div` beside `DialogClose`.'
    },
    schema: []
  },
  create(context) {
    return {
      JSXElement(node) {
        if (nameOf(node.openingElement) !== 'DialogDescription') return
        if (enclosingHeader(node)) context.report({ node, messageId: 'inHeader' })
      }
    }
  }
}
