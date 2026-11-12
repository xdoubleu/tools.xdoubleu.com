import type { KnipConfig } from 'knip'

const config: KnipConfig = {
  entry: ['app/**/*.{ts,tsx}', 'lib/client.ts', 'components/**/*.{ts,tsx}', 'scripts/*.mjs'],
  project: [
    'app/**/*.{ts,tsx}',
    'src/**/*.{ts,tsx}',
    'components/**/*.{ts,tsx}',
    'lib/**/*.{ts,tsx}',
    'hooks/**/*.{ts,tsx}',
    'scripts/*.mjs'
  ],
  ignore: ['lib/gen/**'],
  ignoreDependencies: [
    '@bufbuild/protoc-gen-es',
    // Used via CSS @import/@plugin directives in app/globals.css — knip cannot trace CSS imports
    '@tailwindcss/typography',
    'tailwindcss'
  ],
  ignoreBinaries: ['buf', 'psql', 'go']
}

export default config
