import cardPadding from './card-padding.mjs'
import controlOverrides from './control-overrides.mjs'
import dialogDescription from './dialog-description.mjs'
import fieldLabels from './field-labels.mjs'
import mobileFirstClasses from './mobile-first-classes.mjs'
import noClassnameTemplate from './no-classname-template.mjs'
import sentenceCase from './sentence-case.mjs'
import sharedPatterns from './shared-patterns.mjs'
import themeTokens from './theme-tokens.mjs'
import touchTargets from './touch-targets.mjs'
import usePrimitives from './use-primitives.mjs'

/** Local `ui/*` rules enforcing docs/convention-ui-standards.md. */
export default {
  meta: { name: 'ui' },
  rules: {
    'card-padding': cardPadding,
    'control-overrides': controlOverrides,
    'dialog-description': dialogDescription,
    'field-labels': fieldLabels,
    'mobile-first-classes': mobileFirstClasses,
    'no-classname-template': noClassnameTemplate,
    'sentence-case': sentenceCase,
    'shared-patterns': sharedPatterns,
    'theme-tokens': themeTokens,
    'touch-targets': touchTargets,
    'use-primitives': usePrimitives
  }
}
