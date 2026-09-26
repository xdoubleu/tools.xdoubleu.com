import rule from './mobile-first-classes.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/mobile-first-classes', rule, {
  valid: [
    '<div className="grid grid-cols-1 sm:grid-cols-3" />',
    '<div className="grid-cols-2" />',
    '<div className="min-h-dvh h-[85dvh]" />',
    '<div className={cn("w-full", open && "lg:grid-cols-4")} />',
    '<div style={{ width: 48 }} />',
    '<BookCover style={{ width: 400 }} />',
    'const x = `h-${size}`'
  ],
  invalid: [
    { code: '<div className="grid grid-cols-3" />', errors: [{ messageId: 'gridCols' }] },
    {
      code: '<div className={cn("a", x ? "grid-cols-4" : "")} />',
      errors: [{ messageId: 'gridCols' }]
    },
    { code: '<div className="min-h-screen" />', errors: [{ messageId: 'screen' }] },
    { code: '<div className={`h-screen ${x}`} />', errors: [{ messageId: 'screen' }] },
    { code: '<div className="max-h-[85vh]" />', errors: [{ messageId: 'vh' }] },
    { code: '<div className="h-[calc(100vh-4rem)]" />', errors: [{ messageId: 'vh' }] },
    { code: '<div className="min-w-[600px]" />', errors: [{ messageId: 'px' }] },
    { code: 'const panelClass = "w-[320px]"', errors: [{ messageId: 'px' }] },
    { code: 'const x = cn("h-screen")', errors: [{ messageId: 'screen' }] },
    { code: '<video style={{ width: 200 }} />', errors: [{ messageId: 'style' }] },
    { code: '<div style={{ minWidth: "600px" }} />', errors: [{ messageId: 'style' }] }
  ]
})
