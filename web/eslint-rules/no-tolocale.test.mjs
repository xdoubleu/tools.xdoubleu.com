import rule from './no-tolocale.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/no-tolocale', rule, {
  valid: [
    'formatDate(value)',
    'const s = n.toFixed(1)',
    'new Intl.DateTimeFormat("en-GB").format(d)'
  ],
  invalid: [
    { code: 'd.toLocaleDateString("en-GB")', errors: [{ messageId: 'banned' }] },
    { code: 'n.toLocaleString()', errors: [{ messageId: 'banned' }] },
    { code: 'new Date(x).toLocaleTimeString([], {})', errors: [{ messageId: 'banned' }] }
  ]
})
