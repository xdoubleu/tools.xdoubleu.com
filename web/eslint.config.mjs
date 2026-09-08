// @ts-check
import nextPlugin from '@next/eslint-plugin-next'
import typescriptEslint from 'typescript-eslint'
import js from '@eslint/js'

// Mechanically-checkable mobile-first violations from docs/convention-ui-standards.md
// and the mobile-review skill — each is a class-string shape a Playwright
// audit can only guess at from source, so it's enforced here instead.
// grid-cols-2 is intentionally excluded: components/ui/stat.tsx's
// StatTileGrid deliberately runs two-up on mobile, and that pattern is
// reused (BooksDashboardView, GamesDashboardView, GamesStatsPanel) as an
// established design choice, not a bug.
const mobileFirstRestrictedSyntax = [
  {
    selector:
      "JSXAttribute[name.name='className'] > Literal[value=/(^|\\s)grid-cols-(3|4|5|6|7|8|9|10|11|12)(\\s|$)/]",
    message:
      'Bare grid-cols-N (N≥3) with no sm:/md: prefix starts at N columns on a phone. Write grid-cols-1 sm:grid-cols-N instead.'
  },
  {
    selector: "JSXAttribute[name.name='className'] > Literal[value=/(^|\\s)h-screen(\\s|$)/]",
    message:
      'h-screen is taller than the visible area once mobile browser chrome collapses. Use h-dvh (or min-h-dvh) instead.'
  },
  {
    selector:
      "JSXAttribute[name.name='className'] > Literal[value=/\\b(w|min-w|max-w|h)-\\[\\d+px\\]/]",
    message:
      'Fixed-pixel width/height breaks at narrow viewports. Use relative units (%, rem, fr) or a responsive class instead.'
  }
]

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
        },
        ...mobileFirstRestrictedSyntax
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
        },
        ...mobileFirstRestrictedSyntax
      ]
    }
  },
  {
    // Build/codegen scripts run under Node, not in the browser.
    files: ['scripts/**/*.mjs'],
    languageOptions: {
      globals: {
        console: 'readonly',
        process: 'readonly',
        URL: 'readonly'
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
