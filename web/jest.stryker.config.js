// Jest config used only by StrykerJS. Excludes tests with a per-file
// `@jest-environment` docblock: Stryker's coverage hook only works in its own
// jest-env mixins, so they abort the dry run with "Missing coverage results".
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
