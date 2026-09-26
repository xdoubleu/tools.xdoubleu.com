import rule from './touch-targets.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/touch-targets', rule, {
  valid: [
    '<Button size="sm" className="sm:h-8" />',
    '<Button className="h-auto min-h-11 w-full" />',
    '<Button variant="link" className="h-auto" />',
    '<Input className="min-w-0 flex-1" />',
    '<Input className="w-full sm:w-40" />',
    '<Input type="number" inputMode="numeric" />',
    '<Input className="h-14 text-2xl md:text-sm" />',
    '<iframe title="Preview" />',
    '<SectionCard title="Stats" />',
    '<div className="h-6 w-6" />'
  ],
  invalid: [
    { code: '<Button className="h-8" />', errors: [{ messageId: 'small' }] },
    { code: '<ToggleIconButton className={cn("size-6")} />', errors: [{ messageId: 'small' }] },
    { code: '<Button className="h-auto p-0" />', errors: [{ messageId: 'auto' }] },
    { code: '<Input className="h-9" />', errors: [{ messageId: 'small' }] },
    { code: '<Select className="text-sm" />', errors: [{ messageId: 'fieldText' }] },
    { code: '<Select className="w-28" />', errors: [{ messageId: 'fieldWidth' }] },
    { code: '<Input type="number" />', errors: [{ messageId: 'inputMode' }] },
    { code: '<span title="Details" />', errors: [{ messageId: 'title' }] },
    { code: '<TableCell title={err} />', errors: [{ messageId: 'title' }] }
  ]
})
