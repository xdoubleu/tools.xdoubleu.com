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
  ignoreDependencies: [
    '@bufbuild/protoc-gen-es',
    'eslint-config-next',
    '@next/eslint-plugin-next',
    // Used via CSS @import/@plugin directives in app/globals.css — knip cannot trace CSS imports
    '@tailwindcss/typography',
    'tailwindcss'
  ],
  ignoreBinaries: ['buf']
}

export default config
