'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useTodos } from '@/hooks/useTodos'
import { useTodoSettings } from '@/hooks/useTodoSettings'
import { TaskCard } from '@/components/todos/TaskCard'
import { PoliciesBanner } from '@/components/todos/PoliciesBanner'
import { getApiUrl } from '@/lib/env'

type Tab = 'active' | 'done' | 'archive' | 'search'

export default function TodosPage() {
  const [activeTab, setActiveTab] = useState<Tab>('active')
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState<string | undefined>(undefined)
  const [selectedSectionId, setSelectedSectionId] = useState<string | undefined>(undefined)
  const [quickAddInput, setQuickAddInput] = useState('')
  const [searchQuery, setSearchQuery] = useState('')

  const status = activeTab === 'done' ? 'done' : activeTab === 'archive' ? 'archived' : 'open'

  const { data, error, isLoading, mutate } = useTodos({
    workspaceId: selectedWorkspaceId,
    sectionId: selectedSectionId,
    status: activeTab === 'active' ? undefined : status
  })

  const { data: settings } = useTodoSettings()

  const tasks = data?.tasks ?? []
  const sections = settings?.sections ?? []
  const workspaces = settings?.workspaces ?? []
  const policies = settings?.policies ?? []

  const filteredTasks =
    activeTab === 'search' && searchQuery.trim()
      ? tasks.filter((t) => t.title.toLowerCase().includes(searchQuery.trim().toLowerCase()))
      : tasks

  async function handleQuickAdd(e: React.FormEvent) {
    e.preventDefault()
    const input = quickAddInput.trim()
    if (!input) return

    const form = new FormData()
    form.append('Input', input)
    if (selectedSectionId) form.append('SectionID', selectedSectionId)

    try {
      await fetch(`${getApiUrl()}/todos/`, { method: 'POST', body: form })
      setQuickAddInput('')
      await mutate()
    } catch {
      // Silently ignore network errors in this UI
    }
  }

  const TABS: { id: Tab; label: string }[] = [
    { id: 'active', label: 'Active' },
    { id: 'done', label: 'Done' },
    { id: 'archive', label: 'Archive' },
    { id: 'search', label: 'Search' }
  ]

  return (
    <div className="flex gap-6">
      {/* Left sidebar */}
      <aside className="w-48 shrink-0">
        {/* Workspaces */}
        {workspaces.length > 0 && (
          <section className="mb-4">
            <h2 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted">
              Workspaces
            </h2>
            <ul>
              <li>
                <button
                  type="button"
                  onClick={() => {
                    setSelectedWorkspaceId(undefined)
                    setSelectedSectionId(undefined)
                  }}
                  className={`w-full rounded-lg px-2 py-2 text-left text-sm transition-colors ${selectedWorkspaceId === undefined ? 'bg-accent/10 font-medium text-accent' : 'text-subtle hover:bg-surface'}`}
                >
                  All workspaces
                </button>
              </li>
              {workspaces.map((ws) => (
                <li key={ws.id}>
                  <button
                    type="button"
                    onClick={() => {
                      setSelectedWorkspaceId(ws.id)
                      setSelectedSectionId(undefined)
                    }}
                    className={`w-full rounded-lg px-2 py-2 text-left text-sm transition-colors ${selectedWorkspaceId === ws.id ? 'bg-accent/10 font-medium text-accent' : 'text-subtle hover:bg-surface'}`}
                  >
                    {ws.name}
                  </button>
                </li>
              ))}
            </ul>
          </section>
        )}

        {/* Sections */}
        {sections.length > 0 && (
          <section>
            <h2 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted">
              Sections
            </h2>
            <ul>
              <li>
                <button
                  type="button"
                  onClick={() => setSelectedSectionId(undefined)}
                  className={`w-full rounded-lg px-2 py-2 text-left text-sm transition-colors ${selectedSectionId === undefined ? 'bg-accent/10 font-medium text-accent' : 'text-subtle hover:bg-surface'}`}
                >
                  All sections
                </button>
              </li>
              {sections.map((sec) => (
                <li key={sec.id}>
                  <button
                    type="button"
                    onClick={() => setSelectedSectionId(sec.id)}
                    className={`w-full rounded-lg px-2 py-2 text-left text-sm transition-colors ${selectedSectionId === sec.id ? 'bg-accent/10 font-medium text-accent' : 'text-subtle hover:bg-surface'}`}
                  >
                    {sec.name}
                  </button>
                </li>
              ))}
            </ul>
          </section>
        )}
      </aside>

      {/* Main content */}
      <div className="min-w-0 flex-1">
        {/* Policies banner */}
        {policies.length > 0 && <PoliciesBanner policies={policies} />}

        {/* Tabs */}
        <div className="mb-4 flex gap-1 border-b border-border">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={activeTab === tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`px-4 py-2 text-sm font-medium transition-colors ${activeTab === tab.id ? 'border-b-2 border-accent text-accent' : 'text-muted hover:text-subtle'}`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Search input */}
        {activeTab === 'search' && (
          <div className="mb-4">
            <input
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search tasks…"
              className="w-full rounded border border-input-border bg-input px-3 py-2 text-sm text-input-text"
              aria-label="Search tasks"
            />
          </div>
        )}

        {/* Task list */}
        {isLoading && <p className="py-8 text-center text-sm text-muted">Loading…</p>}
        {error && <p className="py-8 text-center text-sm text-danger">Failed to load tasks.</p>}
        {!isLoading && !error && filteredTasks.length === 0 && (
          <p className="py-8 text-center text-sm text-muted">No tasks.</p>
        )}
        {!isLoading && !error && (
          <div role="list" className="space-y-2">
            {filteredTasks.map((task) => (
              <Link key={task.id} href={`/todos/${task.id}`}>
                <TaskCard task={task} />
              </Link>
            ))}
          </div>
        )}

        {/* Quick add bar */}
        {activeTab === 'active' && (
          <form onSubmit={handleQuickAdd} className="mt-6 flex gap-2" aria-label="Quick add task">
            <input
              type="text"
              value={quickAddInput}
              onChange={(e) => setQuickAddInput(e.target.value)}
              placeholder="Add a task…"
              className="h-11 flex-1 rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            />
            <button
              type="submit"
              className="rounded-xl bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-accent-hover disabled:opacity-50"
              disabled={!quickAddInput.trim()}
            >
              Add
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
