'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { ConnectError, Code } from '@connectrpc/connect'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { useAddBookByURL } from '@/hooks/useBooks'
import { swrKeys } from '@/lib/swrKeys'

const ADD_BY_URL_ERRORS: Partial<Record<Code, string>> = {
  [Code.InvalidArgument]: 'That page has no readable article content (or the URL is invalid).',
  [Code.NotFound]: 'No arXiv paper found for that id.',
  [Code.Unavailable]: 'The page could not be fetched — it may be down or paywalled.',
  [Code.ResourceExhausted]: 'The file behind that URL is too large.'
}

function errorMessage(err: unknown): string {
  if (err instanceof ConnectError) {
    return ADD_BY_URL_ERRORS[err.code] ?? 'Adding the URL failed. Please try again.'
  }
  return 'Adding the URL failed. Please try again.'
}

// AddByUrlForm ingests a pasted URL as a library item: arXiv links become
// papers (metadata + PDF), other pages become readability-extracted article
// EPUBs. Pasting something already in the library is not an error — it just
// reports so. Shared by the unified "Add to library" dialog.
export default function AddByUrlForm({
  onAdded,
  onDone
}: {
  onAdded?: () => void
  // Called once the item is successfully added (and not already present), so a
  // host dialog can close itself.
  onDone?: () => void
}) {
  const addByURL = useAddBookByURL()
  const [url, setUrl] = useState('')
  const [category, setCategory] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const submit = async () => {
    if (!url.trim() || busy) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      const resp = await addByURL(url.trim(), category)
      await mutate(swrKeys.books)
      onAdded?.()
      if (resp.alreadyInLibrary) {
        setNotice('Already in your library.')
      } else {
        setUrl('')
        setCategory('')
        onDone?.()
      }
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form
      className="space-y-3"
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
    >
      <Input
        type="url"
        required
        placeholder="https://arxiv.org/abs/… or an article URL"
        value={url}
        onChange={(e) => setUrl(e.target.value)}
        aria-label="URL"
      />
      <Select value={category} onChange={(e) => setCategory(e.target.value)} aria-label="Category">
        <option value="">Auto-detect category</option>
        <option value="paper">Paper</option>
        <option value="article">Article</option>
      </Select>
      {error && <p className="text-sm text-danger">{error}</p>}
      {notice && <p className="text-sm text-muted">{notice}</p>}
      <div className="flex justify-end">
        <Button type="submit" disabled={busy || !url.trim()}>
          {busy ? 'Adding…' : 'Add'}
        </Button>
      </div>
    </form>
  )
}
