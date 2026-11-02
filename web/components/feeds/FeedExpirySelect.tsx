import { useState } from 'react'
import { useUpdateFeedExpiry } from '@/hooks/useFeedExpiry'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { EXPIRY_OPTIONS, expiryOptionLabel } from '@/lib/feeds/expiry'
import type { Feed } from '@/lib/gen/feeds/v1/feeds_pb'

// How long a feed's unread items live before they expire into the Expired
// view; read and bookmarked items are never affected.
export default function FeedExpirySelect({ feed }: { feed: Feed }) {
  const update = useUpdateFeedExpiry()
  const [busy, setBusy] = useState(false)
  const [failed, setFailed] = useState(false)
  const options: number[] = [...EXPIRY_OPTIONS]
  if (!options.includes(feed.expiryDays)) options.splice(options.length - 1, 0, feed.expiryDays)

  const change = async (days: number) => {
    setBusy(true)
    setFailed(false)
    try {
      await update(feed.id, feed.title, days)
    } catch {
      setFailed(true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Label htmlFor={`expiry-${feed.id}`} className="text-xs font-normal text-muted">
        Unread items expire after
      </Label>
      <Select
        id={`expiry-${feed.id}`}
        value={feed.expiryDays}
        disabled={busy}
        onChange={(e) => void change(Number(e.target.value))}
        className="w-auto"
      >
        {options.map((days) => (
          <option key={days} value={days}>
            {expiryOptionLabel(days)}
          </option>
        ))}
      </Select>
      {failed && (
        <span className="text-xs text-danger" role="alert">
          Saving failed. Please try again.
        </span>
      )}
    </div>
  )
}
