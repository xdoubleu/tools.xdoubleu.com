import type { KnipConfig } from 'knip'

const config: KnipConfig = {
  entry: ['app/**/*.{ts,tsx}', 'lib/client.ts', 'components/**/*.{ts,tsx}'],
  project: [
    'app/**/*.{ts,tsx}',
    'components/**/*.{ts,tsx}',
    'lib/**/*.{ts,tsx}',
    'hooks/**/*.{ts,tsx}'
  ],
  ignore: ['lib/gen/**'],
  ignoreDependencies: ['@bufbuild/protoc-gen-es', '@next/eslint-plugin-next'],
  ignoreBinaries: ['buf']
}

export default config
