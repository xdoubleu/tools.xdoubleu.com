import assert from 'node:assert/strict'
import { test } from 'node:test'
import rule, { themeColorTokens } from './theme-tokens.mjs'
import { ruleTester } from './rule-tester.mjs'

test('reads colour tokens from globals.css', () => {
  const tokens = themeColorTokens()
  for (const t of ['accent', 'muted', 'card', 'input-border', 'star']) assert.ok(tokens.has(t), t)
})

ruleTester.run('ui/theme-tokens', rule, {
  valid: [
    '<div className="bg-card text-muted border border-border ring-1 ring-accent/20" />',
    '<div className="text-sm text-center text-[10px] border-2 border-t bg-cover" />',
    '<div className="bg-black/50 text-white fill-current stroke-2 ring-offset-1" />',
    '<div className="shadow-card shadow-elevated shadow-none divide-y outline-none" />',
    '<div className="text-star hover:text-star/80" />',
    '<Bar fill="var(--color-accent)" />'
  ],
  invalid: [
    { code: '<span className="text-amber-500" />', errors: [{ messageId: 'palette' }] },
    { code: '<div className={cn(ok && "bg-yellow-50")} />', errors: [{ messageId: 'palette' }] },
    { code: '<div className="bg-[#3b82f6]" />', errors: [{ messageId: 'palette' }] },
    { code: '<p className="text-muted-foreground" />', errors: [{ messageId: 'unknown' }] },
    { code: '<div className="border-primary" />', errors: [{ messageId: 'unknown' }] },
    { code: '<div className="dark:bg-card" />', errors: [{ messageId: 'dark' }] },
    { code: '<div className="shadow-sm" />', errors: [{ messageId: 'shadow' }] },
    {
      code: 'const variantClasses = { a: "bg-red-500" }',
      errors: [{ messageId: 'palette' }]
    },
    { code: '<Bar fill="#3b82f6" />', errors: [{ messageId: 'hex' }] }
  ]
})
