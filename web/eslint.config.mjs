// @ts-check
import nextPlugin from '@next/eslint-plugin-next'
import typescriptEslint from 'typescript-eslint'
import js from '@eslint/js'

export default [
  {
    ignores: ['.next', 'node_modules', 'dist', 'lib/gen/**']
  },
  js.configs.recommended,
  ...typescriptEslint.configs.recommended,
  {
    files: ['**/*.{js,jsx,ts,tsx}'],
    plugins: {
      '@next/next': nextPlugin
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
    // Every interactive control goes through a components/ui/ primitive, so
    // focus rings, disabled states and theming stay consistent. See
    // docs/convention-ui-standards.md. components/ui/ itself is exempt below —
    // the primitives are what render the real elements.
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
            'Use Button from @/components/ui/button (or MenuItem/TogglePill/ToggleIconButton) instead of a raw <button>. See docs/spec-ui-primitives.md.'
        },
        {
          selector: "JSXOpeningElement[name.name='input']",
          message:
            'Use Input from @/components/ui/input (or Checkbox/RadioGroup/DateInput) instead of a raw <input>. See docs/spec-ui-primitives.md.'
        },
        {
          selector: "JSXOpeningElement[name.name='select']",
          message:
            'Use Select from @/components/ui/select (or Combobox) instead of a raw <select>. See docs/spec-ui-primitives.md.'
        },
        {
          selector: "JSXOpeningElement[name.name='textarea']",
          message:
            'Use Textarea from @/components/ui/textarea instead of a raw <textarea>. See docs/spec-ui-primitives.md.'
        }
      ]
    }
  },
  {
    // The primitives themselves must render the real elements.
    files: ['components/ui/**/*.tsx'],
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
    // Build/codegen scripts run under Node, not in the browser.
    files: ['scripts/**/*.mjs'],
    languageOptions: {
      globals: {
        console: 'readonly',
        process: 'readonly'
      }
    }
  },
  {
    files: ['*.config.js'],
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
