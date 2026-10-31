// @ts-check
import typescriptEslint from 'typescript-eslint'
import js from '@eslint/js'
import uiPlugin from './eslint-rules/index.mjs'

// ui/* rules (eslint-rules/, docs/convention-ui-standards.md). The first
// three also apply inside components/ui; the rest police call sites.
const uiEverywhere = {
  'ui/mobile-first-classes': 'error',
  'ui/theme-tokens': 'error',
  'ui/no-classname-template': 'error'
}
const uiCallSites = {
  ...uiEverywhere,
  'ui/dialog-description': 'error',
  'ui/field-labels': 'error',
  'ui/touch-targets': 'error',
  'ui/use-primitives': 'error'
}

export default [
  {
    ignores: [
      'node_modules',
      'dist',
      'public',
      'coverage',
      'lib/gen/**',
      '.stryker-tmp',
      'reports',
    ]
  },
  js.configs.recommended,
  ...typescriptEslint.configs.recommended,
  {
    files: ['**/*.{js,jsx,ts,tsx}'],
    plugins: {
      ui: uiPlugin
    },
    rules: {
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
    files: ['components/**/*.tsx', 'app/**/*.tsx', 'src/**/*.tsx'],
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
      'ui/dialog-description': 'off',
      'ui/field-labels': 'off',
      'ui/touch-targets': 'off',
      'ui/use-primitives': 'off'
    }
  },
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
    files: ['*.config.cjs', '.dependency-cruiser.cjs'],
    languageOptions: {
      sourceType: 'commonjs',
      globals: {
        __dirname: 'readonly',
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
