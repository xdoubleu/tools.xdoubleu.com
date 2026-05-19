'use client'

import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import type { GetBooksProgressResponse } from '@/lib/gen/backlog/v1/books_pb'

interface BooksProgressChartProps {
  data: GetBooksProgressResponse | undefined
}

export default function BooksProgressChart({ data }: BooksProgressChartProps) {
  if (!data?.progress?.labels || data.progress.labels.length === 0) {
    return <p className="text-muted">No progress data available.</p>
  }

  const chartData = data.progress.labels.map((label: string, idx: number) => ({
    label: label,
    value: parseInt(data.progress?.values?.[idx] || '0', 10)
  }))

  return (
    <div className="w-full h-64">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="label" width={80} />
          <YAxis />
          <Tooltip />
          <Line type="monotone" dataKey="value" stroke="#3b82f6" />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
