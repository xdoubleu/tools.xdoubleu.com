// Recharts styling shared by every themed chart.

// Stryker disable StringLiteral,ObjectLiteral: chart styling is presentation only.
export const axisTick = { fontSize: 11, fill: 'var(--color-muted)' }

export const tooltipProps = {
  cursor: { fill: 'rgb(var(--hover-rgb) / 0.5)' },
  contentStyle: {
    backgroundColor: 'var(--color-surface)',
    border: '1px solid var(--color-border)',
    borderRadius: '0.75rem',
    color: 'var(--color-fg)'
  },
  labelStyle: { color: 'var(--color-fg)' },
  itemStyle: { color: 'var(--color-fg)' }
}
// Stryker restore StringLiteral,ObjectLiteral
