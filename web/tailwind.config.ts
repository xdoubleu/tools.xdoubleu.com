import type { Config } from 'tailwindcss'

const config: Config = {
  darkMode: 'media',
  content: ['./app/**/*.{ts,tsx}', './components/**/*.{ts,tsx}', './lib/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: 'rgb(var(--color-bg) / <alpha-value>)',
        surface: 'rgb(var(--color-surface) / <alpha-value>)',
        card: 'rgb(var(--color-card) / <alpha-value>)',
        hover: 'rgb(var(--color-hover) / <alpha-value>)',
        fg: 'rgb(var(--color-fg) / <alpha-value>)',
        muted: 'rgb(var(--color-muted) / <alpha-value>)',
        subtle: 'rgb(var(--color-subtle) / <alpha-value>)',
        border: 'rgb(var(--color-border) / <alpha-value>)',
        input: 'rgb(var(--color-input) / <alpha-value>)',
        'input-border': 'rgb(var(--color-input-border) / <alpha-value>)',
        'input-text': 'rgb(var(--color-input-text) / <alpha-value>)',
        accent: 'rgb(var(--color-accent) / <alpha-value>)',
        'accent-hover': 'rgb(var(--color-accent-hover) / <alpha-value>)',
        success: 'rgb(var(--color-success) / <alpha-value>)',
        warn: 'rgb(var(--color-warn) / <alpha-value>)',
        danger: 'rgb(var(--color-danger) / <alpha-value>)',
        glass: 'rgb(var(--color-glass) / var(--glass-opacity))'
      },
      boxShadow: {
        glass: '0 1px 3px 0 rgb(0 0 0 / 0.08), 0 1px 2px -1px rgb(0 0 0 / 0.05)',
        card: '0 1px 3px 0 rgb(0 0 0 / 0.06), 0 1px 2px -1px rgb(0 0 0 / 0.04)',
        elevated: '0 4px 16px 0 rgb(0 0 0 / 0.10), 0 2px 4px -1px rgb(0 0 0 / 0.06)'
      },
      borderRadius: {
        xl2: '1rem',
        xl3: '1.5rem'
      }
    }
  },
  plugins: []
}

export default config
