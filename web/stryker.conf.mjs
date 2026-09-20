// StrykerJS config (issue #1632). `npm run test:mutation` runs the full,
// un-scoped baseline; `npm run test:mutation:diff`
// (scripts/test-mutation-diff.sh) scopes `mutate` to files changed vs
// origin/main instead of overriding it here.
/** @type {import('@stryker-mutator/api/core').PartialStrykerOptions} */
export default {
  packageManager: 'npm',
  testRunner: 'jest',
  jest: {
    // See jest.stryker.config.js for why StrykerJS uses its own wrapper
    // config instead of jest.config.js directly.
    configFile: 'jest.stryker.config.js',
    enableFindRelatedTests: true
  },
  reporters: ['clear-text', 'progress', 'html'],
  coverageAnalysis: 'perTest',
  // Mirrors jest.config.js's collectCoverageFrom allowlist/excludes -- the
  // same files diff_coverage_ts.py already treats as coverage-relevant.
  // Pinned to @stryker-mutator/core@9.6.1 (not the newer major) because its
  // bundled @babel/generator ~8.0.0 crashes instrumenting a plain TS
  // function-type parameter (e.g. `call: () => Promise<T>`, found in
  // hooks/useBooks.ts and elsewhere) -- an upstream instrumenter bug,
  // reproduced on several files across components/, lib/ and hooks/.
  // 9.6.1's @babel/generator ~7.29.0 does not hit it.
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
    // next/dynamic's babel/SWC transform requires its options argument to
    // stay a plain object literal ("next/dynamic options must be an object
    // literal"); Stryker's instrumentation wraps that literal in a
    // conditional expression, which fails that check at Jest's Next.js
    // transform step. The only file in the tree using next/dynamic.
    '!components/books/BookPreviewDialog.tsx'
  ],
  incremental: true,
  incrementalFile: '.stryker-tmp/incremental.json'
}
