'use client'

import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import type { GetSteamDistributionResponse } from '@/lib/gen/backlog/v1/games_pb'

interface SteamDistributionChartProps {
  data: GetSteamDistributionResponse | undefined
}

export default function SteamDistributionChart({ data }: SteamDistributionChartProps) {
  if (!data?.data?.games || data.data.games.length === 0) {
    return <p className="text-muted">No distribution data available.</p>
  }

  const chartData = data.data.games.map((game) => ({
    range: game.name,
    completionRate: game.completionRate || '0%'
  }))

  return (
    <div className="w-full h-64">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="range" width={80} />
          <YAxis />
          <Tooltip />
          <Bar dataKey="completionRate" fill="#3b82f6" />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
