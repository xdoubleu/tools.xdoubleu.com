import rule from './control-overrides.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/control-overrides', rule, {
  valid: [
    '<Button size="sm" className="gap-2" />',
    '<Button className="h-auto min-h-11 w-full whitespace-normal py-2" />',
    '<Button variant="link" className="text-xs" />',
    '<Input className="min-w-0 flex-1" />',
    '<Badge className="text-xs" />',
    '<div className="h-8 py-1 text-sm" />'
  ],
  invalid: [
    {
      code: '<Button className="h-8 text-xs" />',
      errors: [{ messageId: 'control' }, { messageId: 'control' }]
    },
    {
      code: '<Button className={cn("sm:h-7", x && "py-1")} />',
      errors: [{ messageId: 'control' }, { messageId: 'control' }]
    },
    { code: '<Input className="h-14" />', errors: [{ messageId: 'control' }] },
    { code: '<Select className="md:text-sm" />', errors: [{ messageId: 'control' }] },
    { code: '<Button className="h-[30px]" />', errors: [{ messageId: 'control' }] }
  ]
})
