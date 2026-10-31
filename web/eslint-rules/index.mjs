import dialogDescription from './dialog-description.mjs'
import fieldLabels from './field-labels.mjs'
import mobileFirstClasses from './mobile-first-classes.mjs'
import noClassnameTemplate from './no-classname-template.mjs'
import themeTokens from './theme-tokens.mjs'
import touchTargets from './touch-targets.mjs'
import usePrimitives from './use-primitives.mjs'

/** Local `ui/*` rules enforcing docs/convention-ui-standards.md. */
export default {
  meta: { name: 'ui' },
  rules: {
    'dialog-description': dialogDescription,
    'field-labels': fieldLabels,
    'mobile-first-classes': mobileFirstClasses,
    'no-classname-template': noClassnameTemplate,
    'theme-tokens': themeTokens,
    'touch-targets': touchTargets,
    'use-primitives': usePrimitives
  }
}
