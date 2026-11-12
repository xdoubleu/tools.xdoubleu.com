// Recharts styling shared by every themed chart.

// Stryker disable StringLiteral,ObjectLiteral,BooleanLiteral: chart styling is presentation only.
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

export const gridProps = { strokeDasharray: '3 3', stroke: 'var(--color-border)' }

/** Count axes: whole numbers, narrow gutter. */
export const countAxisProps = { allowDecimals: false, tick: axisTick, width: 32 }

/** Tooltip for line charts, whose cursor is a rule rather than a bar highlight. */
export const lineTooltipProps = { ...tooltipProps, cursor: { stroke: 'var(--color-border)' } }

/** A smooth, dotless line, as every line chart draws its series. */
export const lineProps = { type: 'monotone', strokeWidth: 2, dot: false } as const
// Stryker restore StringLiteral,ObjectLiteral,BooleanLiteral

/** The nth categorical series colour from `--series-*` in globals.css (1-6). */
export const seriesColor = (n: number) => `rgb(var(--series-${n}-rgb))`
