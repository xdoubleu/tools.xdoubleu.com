// `npm run test:mutation` runs the full baseline;
// scripts/test-mutation-diff.sh scopes `mutate` to changed files.
/** @type {import('@stryker-mutator/api/core').PartialStrykerOptions} */
export default {
  packageManager: 'npm',
  testRunner: 'jest',
  jest: {
    // See jest.stryker.config.js.
    configFile: 'jest.stryker.config.js',
    enableFindRelatedTests: true
  },
  reporters: ['clear-text', 'progress', 'html'],
  coverageAnalysis: 'perTest',
  // Mirrors jest.config.js's collectCoverageFrom. Pinned to core@9.6.1: the
  // next major's @babel/generator 8 crashes on TS function-type parameters.
  mutate: [
    'components/**/*.{ts,tsx}',
    'lib/**/*.{ts,tsx}',
    'hooks/**/*.{ts,tsx}',
    'app/**/*.{ts,tsx}',
    'instrumentation-client.ts',
    '!lib/gen/**',
    '!app/**/apple-icon.tsx',
    '!app/**/icon.tsx',
    '!app/manifest.ts',
    '!app/layout.tsx',
    '!**/*.d.ts',
    // Instrumentation breaks next/dynamic's "options must be an object literal"
    // check; the only next/dynamic user.
    '!components/books/BookPreviewDialog.tsx'
  ],
  incremental: true,
  incrementalFile: '.stryker-tmp/incremental.json'
}
