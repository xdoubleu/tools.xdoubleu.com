const FIELDS = new Set(['Input', 'Textarea', 'Select', 'Combobox', 'DateInput'])
const CONTROLS = new Set([
  ...FIELDS,
  'Checkbox',
  'Radio',
  'RadioGroupItem',
  'input',
  'select',
  'textarea'
])
const LABELS = new Set(['Field', 'Label', 'label'])

const attrOf = (opening, name) =>
  opening.attributes.find((a) => a.type === 'JSXAttribute' && a.name.name === name)
const hasSpread = (opening) => opening.attributes.some((a) => a.type === 'JSXSpreadAttribute')
const nameOf = (opening) => (opening.name.type === 'JSXIdentifier' ? opening.name.name : '')

function wrapsControl(element) {
  return element.children.some(
    (c) => c.type === 'JSXElement' && (CONTROLS.has(nameOf(c.openingElement)) || wrapsControl(c))
  )
}

function insideLabel(opening) {
  for (let p = opening.parent.parent; p; p = p.parent) {
    if (p.type === 'JSXElement' && ['Label', 'label'].includes(nameOf(p.openingElement)))
      return true
  }
  return false
}

/** Form fields need an accessible name; a placeholder is not one (docs/convention-ui-standards.md). */
export default {
  meta: {
    type: 'problem',
    docs: { description: 'Give every form field a label or aria-label' },
    messages: {
      unlabelled:
        '<{{element}}> has no accessible name. Wrap it in `Field` (htmlFor matching its id), or add `aria-label` when there is no visible label. A placeholder is not a label.',
      orphanLabel:
        '`Label` without `htmlFor` labels nothing. Point `htmlFor` at the control’s `id` (or use `Field`); for a group heading, give it an `id` and name the group with `aria-labelledby`.'
    },
    schema: []
  },
  create(context) {
    const source = context.sourceCode
    // `id="a"`, `id={'a'}` and `htmlFor={a}` compare by value or by source text.
    const valueText = (attr) => {
      let v = attr.value
      if (v?.type === 'JSXExpressionContainer') v = v.expression
      if (!v) return ''
      return v.type === 'Literal' ? JSON.stringify(v.value) : source.getText(v)
    }
    const fields = []
    const labels = []
    const targets = new Set()
    const groupNames = new Set()
    // Every id an `aria-labelledby` may hold: each token of a string, each branch of a condition.
    const addIdRefs = (v) => {
      if (!v) return
      if (v.type === 'JSXExpressionContainer') return addIdRefs(v.expression)
      if (v.type === 'Literal' && typeof v.value === 'string') {
        v.value.split(/\s+/).forEach((id) => groupNames.add(JSON.stringify(id)))
      } else if (v.type === 'ConditionalExpression') {
        addIdRefs(v.consequent)
        addIdRefs(v.alternate)
      } else if (v.type === 'LogicalExpression') {
        addIdRefs(v.left)
        addIdRefs(v.right)
      } else groupNames.add(source.getText(v))
    }

    return {
      JSXOpeningElement(node) {
        const name = nameOf(node)
        if (LABELS.has(name)) {
          const htmlFor = attrOf(node, 'htmlFor')
          if (htmlFor) targets.add(valueText(htmlFor))
          else if (name === 'Label' && !hasSpread(node) && !wrapsControl(node.parent)) {
            labels.push(node)
          }
        }
        if (FIELDS.has(name)) fields.push(node)
        const labelledBy = attrOf(node, 'aria-labelledby')
        if (labelledBy) addIdRefs(labelledBy.value)
      },
      'Program:exit'() {
        for (const node of labels) {
          const id = attrOf(node, 'id')
          if (id && groupNames.has(valueText(id))) continue
          context.report({ node, messageId: 'orphanLabel' })
        }
        for (const node of fields) {
          // A spread may carry the name; an explicit id is still checked.
          if (hasSpread(node) && !attrOf(node, 'id')) continue
          const type = attrOf(node, 'type')
          if (type && valueText(type) === '"hidden"') continue
          if (attrOf(node, 'aria-label') || attrOf(node, 'aria-labelledby')) continue
          const id = attrOf(node, 'id')
          if (id && targets.has(valueText(id))) continue
          if (insideLabel(node)) continue
          context.report({ node, messageId: 'unlabelled', data: { element: nameOf(node) } })
        }
      }
    }
  }
}
