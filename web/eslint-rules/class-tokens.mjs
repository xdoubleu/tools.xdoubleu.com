// Shared class-string extraction for the ui/* rules
// (docs/convention-ui-standards.md).

const CLASS_HELPERS = new Set(['cn', 'clsx', 'twMerge'])
const CLASS_PROP = /^(className|[a-z]\w*ClassName)$/
const CLASS_VARIABLE = /(Class|ClassName|Classes)$/
const BREAKPOINTS = new Set(['sm', 'md', 'lg', 'xl', '2xl'])

/** Splits `hover:sm:!bg-card/50` into its variants and base utility. */
export function parseToken(token) {
  // Variants are separated by `:` outside brackets.
  const parts = []
  let depth = 0
  let start = 0
  for (let i = 0; i < token.length; i++) {
    const c = token[i]
    if (c === '[' || c === '(') depth++
    else if (c === ']' || c === ')') depth--
    else if (c === ':' && depth === 0) {
      parts.push(token.slice(start, i))
      start = i + 1
    }
  }
  let base = token.slice(start)
  if (base.startsWith('!')) base = base.slice(1)
  if (base.endsWith('!')) base = base.slice(0, -1)
  return { variants: parts, base, responsive: parts.some((v) => BREAKPOINTS.has(v)) }
}

function split(text) {
  return text.split(/\s+/).filter(Boolean)
}

/**
 * Class tokens reachable from an expression: string and template literals,
 * cn()/clsx() arguments, conditional/logical branches, arrays and object keys.
 * Template tokens touching a `${}` boundary are partial, so they're dropped.
 */
export function collectTokens(node, out = []) {
  if (!node) return out
  switch (node.type) {
    case 'JSXExpressionContainer':
      return collectTokens(node.expression, out)
    case 'Literal':
      if (typeof node.value === 'string') {
        for (const t of split(node.value)) out.push({ token: t, node })
      }
      return out
    case 'TemplateLiteral':
      node.quasis.forEach((q, i) => {
        const raw = q.value.cooked ?? q.value.raw
        const tokens = split(raw)
        if (tokens.length === 0) return
        const first = i > 0 && !/^\s/.test(raw) ? 1 : 0
        const last =
          i < node.quasis.length - 1 && !/\s$/.test(raw) ? tokens.length - 1 : tokens.length
        for (const t of tokens.slice(first, last)) out.push({ token: t, node })
      })
      node.expressions.forEach((e) => collectTokens(e, out))
      return out
    case 'ConditionalExpression':
      collectTokens(node.consequent, out)
      return collectTokens(node.alternate, out)
    case 'LogicalExpression':
      collectTokens(node.left, out)
      return collectTokens(node.right, out)
    case 'ArrayExpression':
      node.elements.forEach((e) => collectTokens(e, out))
      return out
    case 'ObjectExpression':
      for (const p of node.properties) {
        if (p.type !== 'Property') continue
        if (p.key.type === 'Literal') collectTokens(p.key, out)
        else if (p.key.type === 'Identifier' && !p.computed)
          out.push({ token: p.key.name, node: p })
      }
      return out
    case 'CallExpression':
      if (node.callee.type === 'Identifier' && CLASS_HELPERS.has(node.callee.name)) {
        node.arguments.forEach((a) => collectTokens(a, out))
      }
      return out
    case 'TSAsExpression':
    case 'TSSatisfiesExpression':
      return collectTokens(node.expression, out)
    default:
      return out
  }
}

/** The JSX element name (`Button`, `div`, `Foo.Bar`) owning an attribute. */
export function elementName(attr) {
  const n = attr.parent.name
  if (n.type === 'JSXIdentifier') return n.name
  if (n.type === 'JSXMemberExpression') return n.property.name
  return ''
}

export function isClassAttribute(attr) {
  return attr.name.type === 'JSXIdentifier' && CLASS_PROP.test(attr.name.name)
}

/**
 * Calls `onClassList(tokens, { node, element })` once per class source:
 * each className-like JSX attribute, each cn()/clsx() call outside one, and
 * each `*Class`/`*ClassName` variable initialiser. `element` is set for
 * attributes only.
 */
export function visitClassLists(onClassList) {
  const inAttr = (node) => {
    for (let p = node.parent; p; p = p.parent) {
      if (p.type === 'JSXAttribute') return isClassAttribute(p)
      if (p.type === 'VariableDeclarator') return CLASS_VARIABLE.test(p.id?.name ?? '')
    }
    return false
  }
  return {
    JSXAttribute(attr) {
      if (!isClassAttribute(attr) || !attr.value) return
      onClassList(collectTokens(attr.value), { node: attr, element: elementName(attr), attr })
    },
    CallExpression(call) {
      if (call.callee.type !== 'Identifier' || !CLASS_HELPERS.has(call.callee.name)) return
      if (inAttr(call)) return
      // A nested helper call is collected by its outermost helper.
      for (let p = call.parent; p; p = p.parent) {
        if (
          p.type === 'CallExpression' &&
          p.callee.type === 'Identifier' &&
          CLASS_HELPERS.has(p.callee.name)
        )
          return
      }
      onClassList(collectTokens(call), { node: call })
    },
    VariableDeclarator(decl) {
      if (decl.id.type !== 'Identifier' || !CLASS_VARIABLE.test(decl.id.name) || !decl.init) return
      if (decl.init.type === 'CallExpression') return // handled as a helper call
      // A class map (`const sizeClasses = { sm: '…' }`) holds classes in its values.
      const init = decl.init.type === 'TSAsExpression' ? decl.init.expression : decl.init
      const sources =
        init.type === 'ObjectExpression'
          ? init.properties.filter((p) => p.type === 'Property').map((p) => p.value)
          : [init]
      onClassList(
        sources.flatMap((s) => collectTokens(s)),
        { node: decl }
      )
    }
  }
}

/** Value of a static JSX attribute on the same element, if any. */
export function siblingAttr(attr, name) {
  const found = attr.parent.attributes.find(
    (a) => a.type === 'JSXAttribute' && a.name.name === name
  )
  if (!found) return undefined
  if (!found.value) return true
  if (found.value.type === 'Literal') return found.value.value
  if (found.value.type === 'JSXExpressionContainer' && found.value.expression.type === 'Literal') {
    return found.value.expression.value
  }
  return null // present but dynamic
}
