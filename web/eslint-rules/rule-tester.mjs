import { RuleTester } from 'eslint'

/** RuleTester for JSX fixtures, run under `node --test`. */
export const ruleTester = new RuleTester({
  languageOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    parserOptions: { ecmaFeatures: { jsx: true } }
  }
})
