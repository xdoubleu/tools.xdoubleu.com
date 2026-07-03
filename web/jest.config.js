const nextJest = require('next/jest')

const createJestConfig = nextJest({
  dir: './'
})

const customJestConfig = {
  setupFilesAfterEnv: ['<rootDir>/jest.setup.ts'],
  testEnvironment: 'jest-environment-jsdom',
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/$1',
    '^(.+)\\.js$': '$1'
  },
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx', 'json'],
  collectCoverageFrom: [
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
  coverageReporters: ['text', 'lcov']
}

module.exports = createJestConfig(customJestConfig)
