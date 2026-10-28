import rule from './use-primitives.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/use-primitives', rule, {
  valid: [
    '<PageHeader title="Recipes" />',
    '<Card variant="inset" />',
    '<Badge variant="secondary" />',
    '<PageContainer size="narrow" className="space-y-4" />',
    '<Link href="/x" className="block min-w-0" />',
    '<p className="text-muted">Nothing to show</p>',
    '<p>Notes</p>',
    '<EmptyState>No books yet.</EmptyState>',
    '<button className={cn(interactiveCardClass, "p-4")} />',
    '<div className="rounded-2xl bg-card" />',
    '<p className="text-xs text-danger" role="alert">{error}</p>',
    '<span className="text-danger" aria-live="polite">{status}</span>',
    '<MenuItem className="text-danger hover:bg-danger/10" />',
    '<p className="text-muted hover:text-danger" />'
  ],
  invalid: [
    { code: '<h1 className="text-3xl">Recipes</h1>', errors: [{ messageId: 'raw' }] },
    { code: '<table />', errors: [{ messageId: 'raw' }] },
    { code: '<label htmlFor="x">Name</label>', errors: [{ messageId: 'raw' }] },
    { code: '<div role="tablist" />', errors: [{ messageId: 'tablist' }] },
    {
      code: '<Link href="/b" className="absolute inset-0 rounded-2xl" />',
      errors: [{ messageId: 'stretchedLink' }]
    },
    {
      code: '<Link href="/b" className="text-sm text-accent" />',
      errors: [{ messageId: 'textLink' }]
    },
    {
      code: '<div className="rounded-2xl border border-border bg-card p-4" />',
      errors: [{ messageId: 'card' }]
    },
    {
      code: '<div className="rounded-xl border bg-surface p-3" />',
      errors: [{ messageId: 'inset' }]
    },
    {
      code: '<div className="rounded-xl bg-danger/10 px-4 text-danger" />',
      errors: [{ messageId: 'alert' }]
    },
    {
      code: '<span className="rounded-full px-2 text-xs bg-surface" />',
      errors: [{ messageId: 'badge' }]
    },
    { code: '<p className="text-muted">Loading recipe…</p>', errors: [{ messageId: 'state' }] },
    { code: '<p className="text-muted">No books yet.</p>', errors: [{ messageId: 'empty' }] },
    {
      code: '<Link href="/x" className={cn(interactiveCardClass, "block p-4")} />',
      errors: [{ messageId: 'linkCard' }]
    },
    { code: '<a href="/grafana" className={interactiveCardClass} />', errors: [{ messageId: 'linkCard' }] },
    {
      code: '<p className="text-danger">Failed to load books.</p>',
      errors: [{ messageId: 'state' }]
    },
    { code: '<p className="text-sm text-danger">{error}</p>', errors: [{ messageId: 'dangerText' }] },
    {
      code: '<span className={cn("text-xs", failed ? "text-danger" : "text-muted")} />',
      errors: [{ messageId: 'dangerText' }]
    },
    { code: '<PageContainer className="p-6" />', errors: [{ messageId: 'pageContainer' }] },
    { code: '<PageContainer className="max-w-2xl" />', errors: [{ messageId: 'pageContainer' }] }
  ]
})
