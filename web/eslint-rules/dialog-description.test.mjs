import rule from './dialog-description.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/dialog-description', rule, {
  valid: [
    '<><DialogHeader><DialogTitle>T</DialogTitle><DialogClose /></DialogHeader><DialogDescription>D</DialogDescription></>',
    '<DialogHeader><div><DialogTitle>T</DialogTitle><DialogDescription>D</DialogDescription></div><DialogClose /></DialogHeader>',
    '<DialogHeader><DialogTitle>T</DialogTitle></DialogHeader>',
    '<DialogDescription>Only a description</DialogDescription>',
    '<Card><DialogDescription>D</DialogDescription></Card>',
    '<Header><DialogDescription>Another component named Header</DialogDescription></Header>'
  ],
  invalid: [
    {
      code: '<DialogHeader><DialogTitle>T</DialogTitle><DialogDescription>D</DialogDescription></DialogHeader>',
      errors: [{ messageId: 'inHeader' }]
    },
    {
      code: '<DialogHeader><DialogTitle>T</DialogTitle>{task && <DialogDescription>D</DialogDescription>}</DialogHeader>',
      errors: [{ messageId: 'inHeader' }]
    },
    {
      code: '<DialogHeader>{done ? <DialogDescription>A</DialogDescription> : <DialogDescription>B</DialogDescription>}</DialogHeader>',
      errors: [{ messageId: 'inHeader' }, { messageId: 'inHeader' }]
    },
    {
      code: '<DialogHeader><><DialogDescription>D</DialogDescription></></DialogHeader>',
      errors: [{ messageId: 'inHeader' }]
    },
    {
      code: '<DialogHeader><DialogTitle>T</DialogTitle><DialogDescription className="x">D</DialogDescription><DialogClose /></DialogHeader>',
      errors: [{ messageId: 'inHeader' }]
    }
  ]
})
