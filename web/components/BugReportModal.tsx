'use client'

import { useState, useEffect } from 'react'
import { useCreateBugReport } from '@/hooks/useBugReport'

interface BugReportModalProps {
  isOpen: boolean
  onClose: () => void
}

export default function BugReportModal({ isOpen, onClose }: BugReportModalProps) {
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [page, setPage] = useState('')
  const [consoleLogs, setConsoleLogs] = useState('')
  const [wsLog, setWsLog] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [issueUrl, setIssueUrl] = useState('')

  const createBugReport = useCreateBugReport()

  useEffect(() => {
    if (isOpen && typeof window !== 'undefined') {
      setPage(window.location.pathname)
    }
  }, [isOpen])

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (!title.trim() || !description.trim()) {
      setError('Title and description are required')
      return
    }

    setError(null)
    setIsSubmitting(true)

    try {
      const response = await createBugReport(
        title,
        description,
        page,
        consoleLogs,
        wsLog
      )

      setSuccess(true)
      if (response.issueUrl) {
        setIssueUrl(response.issueUrl)
      }

      // Close modal after 2 seconds
      setTimeout(() => {
        onClose()
        setTitle('')
        setDescription('')
        setConsoleLogs('')
        setWsLog('')
        setSuccess(false)
        setIssueUrl('')
      }, 2000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit bug report')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={onClose}
    >
      <div
        className="bg-card rounded-lg shadow-xl p-4 sm:p-6 w-full mx-4 max-w-md"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-xl font-bold mb-4">Report a Bug</h2>

        {success ? (
          <div className="space-y-4">
            <p className="text-green-600">Thank you! Bug report submitted successfully.</p>
            {issueUrl && (
              <p className="text-sm text-muted">
                View on GitHub:{' '}
                <a
                  href={issueUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-600 hover:text-blue-700 underline"
                >
                  {issueUrl}
                </a>
              </p>
            )}
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-3">
            <div>
              <label htmlFor="title" className="block text-sm font-medium text-subtle mb-1">
                Title *
              </label>
              <input
                id="title"
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Brief summary of the bug"
                className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
                required
              />
            </div>

            <div>
              <label htmlFor="description" className="block text-sm font-medium text-subtle mb-1">
                Description *
              </label>
              <textarea
                id="description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Detailed description of the issue..."
                rows={3}
                className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                required
              />
            </div>

            <div>
              <label htmlFor="page" className="block text-sm font-medium text-subtle mb-1">
                Page
              </label>
              <input
                id="page"
                type="text"
                value={page}
                onChange={(e) => setPage(e.target.value)}
                placeholder="Current page"
                className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>

            <div>
              <label htmlFor="consoleLogs" className="block text-sm font-medium text-subtle mb-1">
                Console Logs
              </label>
              <textarea
                id="consoleLogs"
                value={consoleLogs}
                onChange={(e) => setConsoleLogs(e.target.value)}
                placeholder="Optional console error logs..."
                rows={2}
                className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
              />
            </div>

            <div>
              <label htmlFor="wsLog" className="block text-sm font-medium text-subtle mb-1">
                WebSocket Log
              </label>
              <textarea
                id="wsLog"
                value={wsLog}
                onChange={(e) => setWsLog(e.target.value)}
                placeholder="Optional WebSocket logs..."
                rows={2}
                className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
              />
            </div>

            {error && <p className="text-red-600 text-sm">{error}</p>}

            <div className="flex gap-2 pt-2">
              <button
                type="submit"
                disabled={isSubmitting}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
              >
                {isSubmitting ? 'Submitting...' : 'Submit Report'}
              </button>
              <button
                type="button"
                onClick={onClose}
                className="flex-1 px-4 py-2 bg-surface text-fg rounded hover:bg-border"
              >
                Cancel
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
