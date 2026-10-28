import rule from './field-labels.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/field-labels', rule, {
  valid: [
    '<Input aria-label="Search" placeholder="Search…" />',
    '<Combobox aria-labelledby="heading" />',
    '<Field label="Name" htmlFor="name"><Input id="name" /></Field>',
    '<><Label htmlFor="notes">Notes</Label><Textarea id="notes" /></>',
    '<><Field label="From" htmlFor={fromId}><DateInput id={fromId} /></Field></>',
    '<><Label htmlFor={`qty-${i}`}>Qty</Label><Input id={`qty-${i}`} /></>',
    '<Label>Name <Input /></Label>',
    '<Label><Checkbox /> Done</Label>',
    '<><Label id="ing">Ingredients</Label><div role="group" aria-labelledby="ing" /></>',
    '<div role="group" aria-labelledby={hide ? undefined : labelId}>{!hide && <Label id={labelId}>Ownership</Label>}</div>',
    '<Input type="hidden" name="token" />',
    '<Input {...props} />',
    '<Select {...register("unit")} />',
    '<Field label="Pages" htmlFor="pages"><Input id="pages" {...numberProps} /></Field>'
  ],
  invalid: [
    { code: '<Input placeholder="Search…" />', errors: [{ messageId: 'unlabelled' }] },
    { code: '<Textarea id="notes" />', errors: [{ messageId: 'unlabelled' }] },
    { code: '<Input id="pages" {...numberProps} />', errors: [{ messageId: 'unlabelled' }] },
    {
      code: '<Field label="Name" htmlFor="name"><Input id="other" /></Field>',
      errors: [{ messageId: 'unlabelled' }]
    },
    { code: '<Select value={unit} onChange={setUnit} />', errors: [{ messageId: 'unlabelled' }] },
    { code: '<Combobox value={v} suggestions={s} />', errors: [{ messageId: 'unlabelled' }] },
    { code: '<DateInput value={d} onChange={setD} />', errors: [{ messageId: 'unlabelled' }] },
    { code: '<Label>Ingredients</Label>', errors: [{ messageId: 'orphanLabel' }] },
    { code: '<Label id="ing">Ingredients</Label>', errors: [{ messageId: 'orphanLabel' }] },
    {
      code: '<><Label>Notes</Label><Textarea id="notes" /></>',
      errors: [{ messageId: 'orphanLabel' }, { messageId: 'unlabelled' }]
    }
  ]
})
