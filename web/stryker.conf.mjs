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
  // Mirrors jest.config.js's collectCoverageFrom. package.json's overrides
  // exempt the instrumenter from the global @babel/core 7 pin: a Babel 7 parse
  // printed by its @babel/generator 8 crashes on TS function types and
  // `interface extends`.
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
    '!**/*.d.ts'
  ],
  incremental: true,
  incrementalFile: '.stryker-tmp/incremental.json'
}
