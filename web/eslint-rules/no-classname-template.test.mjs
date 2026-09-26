import rule from './no-classname-template.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/no-classname-template', rule, {
  valid: [
    '<div className="a b" />',
    '<div className={cn("a", on && "b")} />',
    '<div className={`static`} />',
    'const s = `a ${b}`'
  ],
  invalid: [
    { code: '<div className={`a ${on ? "b" : ""}`} />', errors: [{ messageId: 'template' }] },
    { code: '<Link linkClassName={`p-2 ${x}`} />', errors: [{ messageId: 'template' }] }
  ]
})
