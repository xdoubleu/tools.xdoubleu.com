import { Button } from '@/components/ui/button'

export default function SectionTabBar<T extends string>({
  tabs,
  active,
  onChange
}: {
  tabs: { id: T; label: string }[]
  active: T
  onChange: (t: T) => void
}) {
  return (
    <div className="flex gap-2 mb-4">
      {tabs.map((t) => (
        <Button
          key={t.id}
          variant={active === t.id ? 'default' : 'secondary'}
          size="sm"
          onClick={() => onChange(t.id)}
        >
          {t.label}
        </Button>
      ))}
    </div>
  )
}
