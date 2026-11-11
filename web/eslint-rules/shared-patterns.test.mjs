import rule from './shared-patterns.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/shared-patterns', rule, {
  valid: [
    '<SectionHeading level="eyebrow">Tags</SectionHeading>',
    '<h3 className="truncate text-sm font-semibold">Title</h3>',
    '<p className="text-sm text-muted" role="status" />',
    '<p className="text-xs text-danger">{detail}</p>',
    '<ProgressBar value={3} aria-label="Progress" />',
    '<DialogClose>×</DialogClose>',
    '<Button>Remove</Button>',
    '<Button role="menuitem">Edit</Button>',
    '<SegmentedTabs value="a" options={[]} />'
  ],
  invalid: [
    { code: '<Button role="tab">Unread</Button>', errors: [{ messageId: 'tabs' }] },
    { code: '<TogglePill role="tab" label="All" active />', errors: [{ messageId: 'tabs' }] },
    {
      code: '<h2 className="text-lg font-semibold">Stats</h2>',
      errors: [{ messageId: 'heading' }]
    },
    {
      code: '<h3 className="mb-2 text-xs font-semibold uppercase tracking-wide">Tags</h3>',
      errors: [{ messageId: 'heading' }]
    },
    {
      code: '<p className="text-xs text-danger" role="alert">{e}</p>',
      errors: [{ messageId: 'fieldError' }]
    },
    { code: '<div role="progressbar" aria-valuenow={3} />', errors: [{ messageId: 'progress' }] },
    {
      code: '<Button variant="ghost" aria-label="Remove">×</Button>',
      errors: [{ messageId: 'remove' }]
    }
  ]
})
