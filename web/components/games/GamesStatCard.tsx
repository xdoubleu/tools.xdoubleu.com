import { Card } from '@/components/ui/card'

export default function GamesStatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <Card className="p-3">
      <p className="text-xs text-muted">{label}</p>
      <p className="text-xl font-bold mt-0.5">{value}</p>
    </Card>
  )
}
