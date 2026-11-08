import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { useRouter } from '@/hooks/navigation'
import { useCurrentUser } from '@/hooks/useAuth'

type Dashboard = 'reading' | 'games'

const OPTIONS: { value: Dashboard; label: string }[] = [
  { value: 'reading', label: 'Books' },
  { value: 'games', label: 'Games' }
]

/** Switch between the owner's books and games dashboards; hidden without access to both. */
export default function DashboardSwitch({ current }: { current: Dashboard }) {
  const router = useRouter()
  const { data: user } = useCurrentUser()
  const hasBoth =
    !!user &&
    (user.role === 'admin' ||
      (user.appAccess.includes('books') && user.appAccess.includes('games')))
  if (!hasBoth) return null

  return (
    <SegmentedTabs
      aria-label="Dashboard"
      value={current}
      onChange={(next) => {
        if (next !== current) router.push(`/dashboard/${next}`)
      }}
      options={OPTIONS}
    />
  )
}
