// @ts-check
import nextPlugin from '@next/eslint-plugin-next'
import typescriptEslint from 'typescript-eslint'
import js from '@eslint/js'
import uiPlugin from './eslint-rules/index.mjs'
import { legacyUiFiles } from './eslint-rules/legacy-files.mjs'

// ui/* rules (eslint-rules/, docs/convention-ui-standards.md). The first
// three also apply inside components/ui; the rest police call sites.
const uiEverywhere = {
  'ui/mobile-first-classes': 'error',
  'ui/theme-tokens': 'error',
  'ui/no-classname-template': 'error'
}
const uiCallSites = {
  ...uiEverywhere,
  'ui/touch-targets': 'error',
  'ui/use-primitives': 'error'
}
const uiOff = Object.fromEntries(Object.keys(uiCallSites).map((rule) => [rule, 'off']))

export default [
  {
    ignores: ['.next', 'node_modules', 'dist', 'lib/gen/**', '.stryker-tmp', 'reports']
  },
  js.configs.recommended,
  ...typescriptEslint.configs.recommended,
  {
    files: ['**/*.{js,jsx,ts,tsx}'],
    plugins: {
      '@next/next': nextPlugin,
      ui: uiPlugin
    },
    rules: {
      ...nextPlugin.configs.recommended.rules,
      ...nextPlugin.configs['core-web-vitals'].rules,
      'no-restricted-syntax': [
        'error',
        {
          selector: 'TSSatisfiesExpression',
          message: 'Use create(Schema, fields) from @bufbuild/protobuf instead of satisfies.'
        }
      ]
    }
  },
  {
    // Interactive controls go through components/ui/ primitives
    // (docs/convention-ui-standards.md).
    files: ['components/**/*.tsx', 'app/**/*.tsx'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: 'TSSatisfiesExpression',
          message: 'Use create(Schema, fields) from @bufbuild/protobuf instead of satisfies.'
        },
        {
          selector: "JSXOpeningElement[name.name='button']",
          message:
            'Use Button from @/components/ui/button (or MenuItem/TogglePill/ToggleIconButton) instead of a raw <button>. See components/ui/README.md.'
        },
        {
          selector: "JSXOpeningElement[name.name='input']",
          message:
            'Use Input from @/components/ui/input (or Checkbox/RadioGroup/DateInput) instead of a raw <input>. See components/ui/README.md.'
        },
        {
          selector: "JSXOpeningElement[name.name='select']",
          message:
            'Use Select from @/components/ui/select (or Combobox) instead of a raw <select>. See components/ui/README.md.'
        },
        {
          selector: "JSXOpeningElement[name.name='textarea']",
          message:
            'Use Textarea from @/components/ui/textarea instead of a raw <textarea>. See components/ui/README.md.'
        }
      ],
      ...uiCallSites
    }
  },
  {
    files: ['components/ui/**/*.tsx'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: 'TSSatisfiesExpression',
          message: 'Use create(Schema, fields) from @bufbuild/protobuf instead of satisfies.'
        }
      ],
      ...uiEverywhere,
      'ui/touch-targets': 'off',
      'ui/use-primitives': 'off'
    }
  },
  {
    // Brand icons and the splash logo are fixed artwork, not themed UI.
    files: [
      'app/**/apple-icon.tsx',
      'app/**/icon.tsx',
      'app/icon-*/**/*.tsx',
      'app/layout.tsx',
      'components/Splash.tsx'
    ],
    rules: { 'ui/theme-tokens': 'off' }
  },
  // Files not yet migrated to the ui/* rules; each domain PR deletes
  // its entries, and new files are always checked.
  ...(legacyUiFiles.length > 0 ? [{ files: legacyUiFiles, rules: uiOff }] : []),
  {
    files: ['scripts/**/*.mjs', 'eslint-rules/**/*.mjs'],
    languageOptions: {
      globals: {
        console: 'readonly',
        process: 'readonly',
        URL: 'readonly'
      }
    }
  },
  {
    files: ['*.config.js', '.dependency-cruiser.js'],
    languageOptions: {
      sourceType: 'commonjs',
      globals: {
        module: 'writable',
        require: 'readonly'
      }
    },
    rules: {
      '@typescript-eslint/no-require-imports': 'off'
    }
  },
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      parserOptions: {
        project: true
      }
    },
    rules: {
      '@typescript-eslint/no-unsafe-type-assertion': 'error'
    }
  },
  {
    files: ['__tests__/**/*.{ts,tsx}', '**/*.test.{ts,tsx}', '**/*.spec.{ts,tsx}'],
    rules: {
      '@typescript-eslint/no-require-imports': 'off',
      // Tests render bare elements as fixtures; the primitive rules don't apply.
      'no-restricted-syntax': 'off'
    }
  }
]
