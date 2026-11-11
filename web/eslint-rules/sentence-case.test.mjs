import rule from './sentence-case.mjs'
import { ruleTester } from './rule-tester.mjs'

ruleTester.run('ui/sentence-case', rule, {
  valid: [
    '<PageHeader title="Edit recipe" />',
    '<PageHeader title="Movies & series" />',
    '<PageHeader title="Books settings" />',
    '<SectionCard title="Kobo devices" />',
    '<SettingsSection title="Connect Todoist" />',
    '<SectionCard title="Sync from Steam" />',
    '<SectionCard title="MCP setup" />',
    '<PageHeader title={name} />',
    '<PageHeader title={`Edit ${name}`} />',
    '<SectionHeading>Not started</SectionHeading>',
    '<Button title="Edit Recipe" />',
    '<SectionHeading>{count} items</SectionHeading>'
  ],
  invalid: [
    { code: '<PageHeader title="Edit Recipe" />', errors: [{ messageId: 'titleCase' }] },
    { code: '<SectionCard title={"Meal Plan"} />', errors: [{ messageId: 'titleCase' }] },
    { code: '<SettingsSection title="Feed Health Check" />', errors: [{ messageId: 'titleCase' }] },
    { code: '<SectionHeading>In Progress</SectionHeading>', errors: [{ messageId: 'titleCase' }] }
  ]
})
