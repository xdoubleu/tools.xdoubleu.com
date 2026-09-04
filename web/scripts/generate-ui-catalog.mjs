// Generates docs/spec-ui-primitives.md — the inventory of components/ui/ that
// contributors and agents check before building a new component. Generated
// rather than hand-written because the previous hand-written list in
// docs/convention-ui-standards.md silently went stale (issue #1412).
//
// Run: npm run generate:ui-catalog       (write)
//      npm run generate:ui-catalog:check (verify it's committed up to date)

import { readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join, dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const webDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const uiDir = join(webDir, 'components', 'ui')
const outFile = resolve(webDir, '..', 'docs', 'spec-ui-primitives.md')

/** Leading `/** ... *\/` block on a node, flattened to one line. */
function jsDocOf(node, text) {
  const ranges = ts.getLeadingCommentRanges(text, node.pos) ?? []
  const block = ranges
    .map((r) => text.slice(r.pos, r.end))
    .filter((c) => c.startsWith('/**'))
    .pop()
  if (!block) return ''
  return block
    .replace(/^\/\*\*/, '')
    .replace(/\*\/$/, '')
    .split('\n')
    .map((l) => l.replace(/^\s*\*ked?/, '').replace(/^\s*\*\s?/, '').trim())
    .join(' ')
    .replace(/\s+/g, ' ')
    .trim()
}

const hasExportModifier = (node) =>
  (node.modifiers ?? []).some((m) => m.kind === ts.SyntaxKind.ExportKeyword)

/**
 * Exported value names of one source file — covering all three forms used in
 * components/ui/: a trailing `export { A, B }`, an inline `export function A`,
 * and `export default function A`.
 */
function exportsOf(source) {
  const values = new Set()
  for (const statement of source.statements) {
    if (ts.isExportDeclaration(statement) && statement.exportClause) {
      if (!ts.isNamedExports(statement.exportClause)) continue
      for (const element of statement.exportClause.elements) {
        if (statement.isTypeOnly || element.isTypeOnly) continue
        values.add(element.name.text)
      }
    } else if (ts.isFunctionDeclaration(statement) && statement.name) {
      if (hasExportModifier(statement)) values.add(statement.name.text)
    } else if (ts.isVariableStatement(statement) && hasExportModifier(statement)) {
      for (const decl of statement.declarationList.declarations) {
        if (ts.isIdentifier(decl.name)) values.add(decl.name.text)
      }
    }
  }
  return { values }
}

/** Declarations by name, so an exported name can be traced back to its JSDoc. */
function declarationsOf(source) {
  const byName = new Map()
  for (const statement of source.statements) {
    if (ts.isFunctionDeclaration(statement) && statement.name) {
      byName.set(statement.name.text, statement)
    } else if (ts.isVariableStatement(statement)) {
      for (const decl of statement.declarationList.declarations) {
        // The JSDoc sits on the whole statement, not the declarator.
        if (ts.isIdentifier(decl.name)) byName.set(decl.name.text, statement)
      }
    } else if (ts.isInterfaceDeclaration(statement) || ts.isTypeAliasDeclaration(statement)) {
      byName.set(statement.name.text, statement)
    }
  }
  return byName
}

/**
 * Inline a local type alias into a prop's type text, so a table shows
 * `'sm' | 'md' | 'lg'` rather than an opaque local name like `Size`.
 */
function resolveAliases(typeText, byName, source) {
  return typeText.replace(/\b[A-Z][A-Za-z0-9]*\b/g, (word) => {
    const decl = byName.get(word)
    if (!decl || !ts.isTypeAliasDeclaration(decl)) return word
    return decl.type.getText(source).replace(/\s+/g, ' ')
  })
}

/** Props of `<Name>Props`, when it's an interface we can read members off. */
function propsOf(name, byName, text) {
  const decl = byName.get(`${name}Props`)
  if (!decl || !ts.isInterfaceDeclaration(decl)) return [[], '']
  const heritage = (decl.heritageClauses ?? [])
    .flatMap((h) => h.types.map((t) => t.getText(decl.getSourceFile())))
    .join(', ')
  const props = decl.members.filter(ts.isPropertySignature).map((member) => ({
    name: member.name.getText(decl.getSourceFile()),
    optional: member.questionToken !== undefined,
    type: member.type
      ? resolveAliases(
          member.type.getText(decl.getSourceFile()).replace(/\s+/g, ' '),
          byName,
          decl.getSourceFile()
        )
      : 'unknown',
    doc: jsDocOf(member, text)
  }))
  return [props, heritage]
}

// Backslashes must be escaped before pipes, or an input backslash would end up
// escaping the escape and break out of the table cell.
function escapeCell(value) {
  return value.replace(/\\/g, '\\\\').replace(/\|/g, '\\|')
}

const files = readdirSync(uiDir)
  .filter((f) => f.endsWith('.tsx'))
  .sort()

const sections = []

for (const file of files) {
  const path = join(uiDir, file)
  const text = readFileSync(path, 'utf8')
  const source = ts.createSourceFile(path, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
  const { values } = exportsOf(source)
  const byName = declarationsOf(source)
  const isClient = /^\s*['"]use client['"]/m.test(text)

  const entries = []
  for (const name of [...values].sort()) {
    const decl = byName.get(name)
    const [props, heritage] = propsOf(name, byName, text)
    entries.push({ name, doc: decl ? jsDocOf(decl, text) : '', props, heritage })
  }
  if (entries.length === 0) continue

  const lines = [`### \`${file}\`${isClient ? ' — client component' : ''}`, '']
  for (const entry of entries) {
    lines.push(`#### \`${entry.name}\``, '')
    if (entry.doc) lines.push(entry.doc, '')
    if (entry.props.length > 0) {
      lines.push('| Prop | Type | Required | Notes |', '|---|---|---|---|')
      for (const prop of entry.props) {
        lines.push(
          `| \`${prop.name}\` | \`${escapeCell(prop.type)}\` | ${prop.optional ? '' : 'yes'} | ${escapeCell(prop.doc)} |`
        )
      }
      lines.push('')
    }
    if (entry.heritage) lines.push(`Also accepts: \`${entry.heritage}\``, '')
  }
  sections.push(lines.join('\n'))
}

const header = `# Spec: components/ui/ primitives

<!--
GENERATED FILE — do not edit by hand.
Run \`npm run generate:ui-catalog\` from web/ after changing components/ui/.
Source: web/components/ui/*.tsx (JSDoc + exported prop types).
-->

- Generated from: \`web/components/ui/*.tsx\`
- Rule that makes these mandatory: [\`convention-ui-standards.md\`](convention-ui-standards.md)
- Issues: #1412

## What this is

The complete inventory of shared UI primitives. **Check here before building a
new component** — the design system's failure mode is not a missing rule, it's
not knowing what already exists. If nothing here fits, add a primitive rather
than styling a raw element at the call site; ESLint blocks the latter.

Prop tables list each component's own props. "Also accepts" means the remaining
props are forwarded to the underlying element.

## Primitives

`

writeFileSync(outFile, header + sections.join('\n') + '\n')
console.log(`Wrote ${outFile} (${files.length} files, ${sections.length} documented)`)
