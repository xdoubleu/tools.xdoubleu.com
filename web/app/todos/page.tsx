'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useTodos } from '@/hooks/useTodos'
import { useTodoSettings } from '@/hooks/useTodoSettings'
import { TaskCard } from '@/components/todos/TaskCard'
import { PoliciesBanner } from '@/components/todos/PoliciesBanner'

type Tab = 'active' | 'done' | 'archive' | 'search'

export default function TodosPage() {
  const [activeTab, setActiveTab] = useState<Tab>('active')
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState<
    string | undefined
  >(undefined)
  const [selectedSectionId, setSelectedSectionId] = useState<
    string | undefined
  >(undefined)
  const [quickAddInput, setQuickAddInput] = useState('')
  const [searchQuery, setSearchQuery] = useState('')

  const status =
    activeTab === 'done'
      ? 'done'
      : activeTab === 'archive'
        ? 'archived'
        : 'open'

  const { data, error, isLoading, mutate } = useTodos({
    workspaceId: selectedWorkspaceId,
    sectionId: selectedSectionId,
    status: activeTab === 'active' ? undefined : status,
  })

  const { data: settings } = useTodoSettings()

  const tasks = data?.tasks ?? []
  const sections = settings?.sections ?? []
  const workspaces = settings?.workspaces ?? []
  const policies = settings?.policies ?? []

  const filteredTasks =
    activeTab === 'search' && searchQuery.trim()
      ? tasks.filter((t) =>
          t.title.toLowerCase().includes(searchQuery.trim().toLowerCase())
        )
      : tasks

  async function handleQuickAdd(e: React.FormEvent) {
    e.preventDefault()
    const input = quickAddInput.trim()
    if (!input) return

    const form = new FormData()
    form.append('Input', input)
    if (selectedSectionId) form.append('SectionID', selectedSectionId)

    try {
      await fetch(
        `${process.env.NEXT_PUBLIC_API_URL ?? ''}/todos/`,
        { method: 'POST', body: form }
      )
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
    { id: 'search', label: 'Search' },
  ]

  return (
    <div className="flex gap-6">
      {/* Left sidebar */}
      <aside className="w-48 shrink-0">
        {/* Workspaces */}
        {workspaces.length > 0 && (
          <section className="mb-4">
            <h2 className="mb-1 text-xs font-semibold uppercase tracking-wide text-gray-400">
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
                  className={`w-full rounded px-2 py-1 text-left text-sm ${selectedWorkspaceId === undefined ? 'bg-blue-50 font-medium text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}
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
                    className={`w-full rounded px-2 py-1 text-left text-sm ${selectedWorkspaceId === ws.id ? 'bg-blue-50 font-medium text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}
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
            <h2 className="mb-1 text-xs font-semibold uppercase tracking-wide text-gray-400">
              Sections
            </h2>
            <ul>
              <li>
                <button
                  type="button"
                  onClick={() => setSelectedSectionId(undefined)}
                  className={`w-full rounded px-2 py-1 text-left text-sm ${selectedSectionId === undefined ? 'bg-blue-50 font-medium text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}
                >
                  All sections
                </button>
              </li>
              {sections.map((sec) => (
                <li key={sec.id}>
                  <button
                    type="button"
                    onClick={() => setSelectedSectionId(sec.id)}
                    className={`w-full rounded px-2 py-1 text-left text-sm ${selectedSectionId === sec.id ? 'bg-blue-50 font-medium text-blue-700' : 'text-gray-700 hover:bg-gray-100'}`}
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
        <div className="mb-4 flex gap-1 border-b border-gray-200">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={activeTab === tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`px-4 py-2 text-sm font-medium ${activeTab === tab.id ? 'border-b-2 border-blue-500 text-blue-600' : 'text-gray-500 hover:text-gray-700'}`}
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
              className="w-full rounded border border-gray-300 px-3 py-2 text-sm"
              aria-label="Search tasks"
            />
          </div>
        )}

        {/* Task list */}
        {isLoading && (
          <p className="py-8 text-center text-sm text-gray-400">Loading…</p>
        )}
        {error && (
          <p className="py-8 text-center text-sm text-red-500">
            Failed to load tasks.
          </p>
        )}
        {!isLoading && !error && filteredTasks.length === 0 && (
          <p className="py-8 text-center text-sm text-gray-400">No tasks.</p>
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
          <form
            onSubmit={handleQuickAdd}
            className="mt-6 flex gap-2"
            aria-label="Quick add task"
          >
            <input
              type="text"
              value={quickAddInput}
              onChange={(e) => setQuickAddInput(e.target.value)}
              placeholder="Add a task…"
              className="flex-1 rounded border border-gray-300 px-3 py-2 text-sm"
            />
            <button
              type="submit"
              className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
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
