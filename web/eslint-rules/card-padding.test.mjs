import rule from './card-padding.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/card-padding', rule, {
  valid: [
    '<Card padding="sm" className="space-y-2" />',
    '<Card padding="none" className="overflow-hidden" />',
    '<CardContent className="p-0" />',
    '<div className="rounded-xl p-4" />'
  ],
  invalid: [
    { code: '<Card className="p-4" />', errors: [{ messageId: 'padding' }] },
    {
      code: '<Card className={cn("px-4 sm:py-3")} />',
      errors: [{ messageId: 'padding' }, { messageId: 'padding' }]
    },
    { code: '<Card variant="inset" className="rounded-lg" />', errors: [{ messageId: 'radius' }] }
  ]
})
