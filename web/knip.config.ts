import type { KnipConfig } from 'knip'

const config: KnipConfig = {
  entry: ['app/**/*.{ts,tsx}', 'lib/client.ts'],
  project: [
    'app/**/*.{ts,tsx}',
    'components/**/*.{ts,tsx}',
    'lib/**/*.{ts,tsx}',
    'hooks/**/*.{ts,tsx}'
  ],
  ignore: ['lib/gen/**'],
  ignoreDependencies: ['@sentry/nextjs']
}

export default config
