import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { parseToken, visitClassLists } from './class-tokens.mjs'

const GLOBALS_CSS = fileURLToPath(new URL('../app/globals.css', import.meta.url))

/** Colour tokens defined in app/globals.css (`--color-<name>`). */
export function themeColorTokens(css = readFileSync(GLOBALS_CSS, 'utf8')) {
  return new Set([...css.matchAll(/--color-([a-z0-9-]+)\s*:/g)].map((m) => m[1]))
}

/** Shadow tokens defined in app/globals.css (`--shadow-<name>`). */
function themeShadowTokens(css = readFileSync(GLOBALS_CSS, 'utf8')) {
  return new Set([...css.matchAll(/--shadow-([a-z0-9-]+)\s*:/g)].map((m) => m[1]))
}

const KEYWORDS = new Set(['transparent', 'current', 'inherit', 'white', 'black'])
const PALETTE =
  /^(slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|mauve|olive|mist|taupe)-\d{2,3}$/
const SIZE = /^(\d+(\.\d+)?|px|\d+\/\d+|xs|sm|md|lg|xl|[2-9]xl|base|none|auto|full)$/

// Non-colour utilities sharing a colour prefix, per prefix.
const NON_COLOR = {
  text: /^(left|center|right|justify|start|end|wrap|nowrap|balance|pretty|ellipsis|clip|shadow(-.*)?)$/,
  bg: /^(fixed|local|scroll|clip-.*|origin-.*|no-repeat|repeat(-.*)?|cover|contain|center|top|bottom|left|right|(left|right)-(top|bottom)|gradient-.*|linear-.*|radial(-.*)?|conic(-.*)?|blend-.*|size-.*|position-.*)$/,
  border:
    /^(x|y|t|r|b|l|s|e)(-\d+)?$|^(solid|dashed|dotted|double|hidden|collapse|separate|spacing(-.*)?)$/,
  ring: /^(inset|offset-\d+)$/,
  outline: /^(dashed|dotted|double|solid|hidden|offset-\d+)$/,
  divide: /^(x|y)(-\d+|-reverse)?$|^(solid|dashed|dotted|double)$/,
  decoration: /^(solid|double|dotted|dashed|wavy|from-font|clone|slice)$/,
  from: /^\d+%$/,
  via: /^\d+%$/,
  to: /^\d+%$/
}
const PREFIXES = [
  'ring-offset',
  'placeholder',
  'decoration',
  'outline',
  'border-x',
  'border-y',
  'border-t',
  'border-r',
  'border-b',
  'border-l',
  'border',
  'divide',
  'stroke',
  'caret',
  'fill',
  'ring',
  'text',
  'from',
  'via',
  'bg',
  'to'
]

/**
 * Colours come from the theme tokens so both schemes stay consistent; `dark:`
 * follows the OS, not the app's theme setting.
 */
export default {
  meta: {
    type: 'problem',
    docs: { description: 'Colours and shadows must use app/globals.css theme tokens' },
    messages: {
      dark: '`{{token}}`: `dark:` follows the OS, not the in-app theme. Theme tokens already switch per theme.',
      palette:
        '`{{token}}` is a raw palette colour. Use a theme token ({{tokens}}) or add one to app/globals.css.',
      unknown: '`{{token}}` names no theme colour, so it does nothing. Use one of: {{tokens}}.',
      shadow: '`{{token}}`: use a theme shadow ({{shadows}}).',
      hex: '`{{attr}}="{{value}}"` hard-codes a colour. Use `var(--color-<token>)`.'
    },
    schema: []
  },
  create(context) {
    const tokens = themeColorTokens()
    const shadows = themeShadowTokens()
    const tokenList = [...tokens].join(', ')
    const shadowList = [...shadows].map((s) => `shadow-${s}`).join(', ')

    const check = (list) => {
      for (const { token, node } of list) {
        const { variants, base } = parseToken(token)
        if (variants.includes('dark')) {
          context.report({ node, messageId: 'dark', data: { token } })
          continue
        }
        if (base.startsWith('shadow-')) {
          const rest = base.slice(7).split('/')[0]
          if (/^(2?xs|sm|md|lg|xl|2xl)$/.test(rest)) {
            context.report({ node, messageId: 'shadow', data: { token, shadows: shadowList } })
          }
          continue
        }
        const prefix = PREFIXES.find((p) => base.startsWith(`${p}-`))
        if (!prefix) continue
        const rest = base.slice(prefix.length + 1).split('/')[0]
        if (rest.startsWith('[') || rest.startsWith('(')) {
          if (/#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i.test(rest)) {
            context.report({ node, messageId: 'palette', data: { token, tokens: tokenList } })
          }
          continue
        }
        if (tokens.has(rest) || KEYWORDS.has(rest)) continue
        if (PALETTE.test(rest)) {
          context.report({ node, messageId: 'palette', data: { token, tokens: tokenList } })
          continue
        }
        const family = prefix.startsWith('border') ? 'border' : prefix
        if (SIZE.test(rest) || NON_COLOR[family]?.test(rest)) continue
        context.report({ node, messageId: 'unknown', data: { token, tokens: tokenList } })
      }
    }

    const classVisitors = visitClassLists(check)
    return {
      ...classVisitors,
      JSXAttribute(attr) {
        classVisitors.JSXAttribute(attr)
        const name = attr.name.name
        if (!['fill', 'stroke', 'color', 'stopColor', 'backgroundColor'].includes(name)) return
        const value =
          attr.value?.type === 'Literal'
            ? attr.value.value
            : attr.value?.type === 'JSXExpressionContainer' &&
                attr.value.expression.type === 'Literal'
              ? attr.value.expression.value
              : null
        if (typeof value === 'string' && /^#[0-9a-f]{3,8}$/i.test(value)) {
          context.report({ node: attr, messageId: 'hex', data: { attr: name, value } })
        }
      }
    }
  }
}
