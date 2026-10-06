// Jest config used only by StrykerJS. Excludes tests with a per-file
// `@jest-environment` docblock: Stryker's coverage hook only works in its own
// jest-env mixins, so they abort the dry run with "Missing coverage results".
const fs = require('fs')
const path = require('path')
const baseConfig = require('./jest.config.js')

function testsWithEnvironmentDocblock(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) return testsWithEnvironmentDocblock(full)
    if (!/\.test\.tsx?$/.test(entry.name)) return []
    return /@jest-environment\b/.test(fs.readFileSync(full, 'utf8')) ? [full] : []
  })
}

module.exports = async () => {
  const config = await baseConfig()
  const root = path.resolve('.')
  const ignored = testsWithEnvironmentDocblock(path.join(root, '__tests__')).map(
    (file) => '<rootDir>/' + path.relative(root, file).split(path.sep).join('/')
  )
  return {
    ...config,
    testPathIgnorePatterns: [...(config.testPathIgnorePatterns ?? []), ...ignored]
  }
}
