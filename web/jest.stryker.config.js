// A thin wrapper around jest.config.js, used only by StrykerJS
// (stryker.conf.mjs's jest.configFile) -- never by `npm test`/`npm run
// test:cov`, which keep using jest.config.js directly.
//
// A handful of test files opt into a per-file `@jest-environment node`
// docblock (rather than the config-level `jsdom` default): env.test.ts,
// env.server.test.ts, logger.test.ts, middleware.test.ts,
// instrumentation.test.ts, sentry.{edge,server}.config.test.ts,
// app/logs/route.test.ts, app/metrics-route.test.ts,
// lib/books/checksum.test.ts. @stryker-mutator/jest-runner's
// coverage-collection hook is only wired into its own jsdom/node
// environments (see its README's `@stryker-mutator/jest-runner/jest-env/*`
// mixins), not a plain per-file environment override, so the moment one of
// these becomes a related test, Stryker's dry run aborts with "Missing
// coverage results". Excluding them here -- rather than rewriting every
// affected test file to use the mixin -- keeps this StrykerJS-only wrapper
// isolated from a testing convention (`@jest-environment`) used well
// beyond mutation testing (issue #1632).
const baseConfig = require('./jest.config.js')

module.exports = async () => {
  const config = await baseConfig()
  return {
    ...config,
    testPathIgnorePatterns: [
      ...(config.testPathIgnorePatterns ?? []),
      '<rootDir>/__tests__/app/logs/route.test.ts',
      '<rootDir>/__tests__/app/metrics-route.test.ts',
      '<rootDir>/__tests__/instrumentation.test.ts',
      '<rootDir>/__tests__/lib/books/checksum.test.ts',
      '<rootDir>/__tests__/lib/env.server.test.ts',
      '<rootDir>/__tests__/lib/env.test.ts',
      '<rootDir>/__tests__/lib/logger.test.ts',
      '<rootDir>/__tests__/middleware.test.ts',
      '<rootDir>/__tests__/sentry.edge.config.test.ts',
      '<rootDir>/__tests__/sentry.server.config.test.ts'
    ]
  }
}
