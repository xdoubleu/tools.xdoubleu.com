'use client'

import { useState, useEffect, useRef, useCallback } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { useTodos } from '@/hooks/useTodos'
import { useTodoSettings } from '@/hooks/useTodoSettings'
import { TaskCard } from '@/components/todos/TaskCard'
import { PoliciesBanner } from '@/components/todos/PoliciesBanner'
import QuickAddBar from '@/components/todos/QuickAddBar'

type Tab = 'active' | 'done' | 'archive'

export default function TodosPage() {
  const router = useRouter()
  const searchParams = useSearchParams()

  const selectedWorkspaceId = searchParams.get('w') ?? undefined

  const [activeTab, setActiveTab] = useState<Tab>('active')
  const [selectedSectionId, setSelectedSectionId] = useState<string | undefined>(undefined)
  const [searchQuery, setSearchQuery] = useState('')
  const [focusedTaskIndex, setFocusedTaskIndex] = useState<number>(-1)
  const [focusedSectionIndex, setFocusedSectionIndex] = useState<number>(-1)

  const taskRefs = useRef<Array<HTMLAnchorElement | null>>([])
  const sectionRefs = useRef<Array<HTMLButtonElement | null>>([])

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
  const labelPresets = settings?.labelPresets ?? []

  const filteredTasks = searchQuery.trim()
    ? tasks.filter((t) => t.title.toLowerCase().includes(searchQuery.trim().toLowerCase()))
    : tasks

  function selectWorkspace(id: string | undefined) {
    setSelectedSectionId(undefined)
    setFocusedTaskIndex(-1)
    if (id) {
      router.replace(`?w=${id}`)
    } else {
      router.replace('/todos')
    }
  }

  // Arrow key navigation for tasks and sections
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      const target = e.target
      const inInput = target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement

      if (inInput) return

      if (e.key === 'ArrowDown') {
        e.preventDefault()
        if (focusedTaskIndex < filteredTasks.length - 1) {
          const next = focusedTaskIndex + 1
          setFocusedTaskIndex(next)
          taskRefs.current[next]?.focus()
        }
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        if (focusedTaskIndex > 0) {
          const prev = focusedTaskIndex - 1
          setFocusedTaskIndex(prev)
          taskRefs.current[prev]?.focus()
        }
      }
    },
    [focusedTaskIndex, filteredTasks.length]
  )

  const handleSectionKeyDown = useCallback(
    (e: KeyboardEvent) => {
      const target = e.target
      const inInput = target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement

      if (inInput) return

      const allSections = [undefined, ...sections.map((s) => s.id)]

      if (e.key === 'ArrowDown') {
        const focused = sectionRefs.current.some((r) => r === document.activeElement)
        if (!focused) return
        e.preventDefault()
        const next = Math.min(focusedSectionIndex + 1, allSections.length - 1)
        setFocusedSectionIndex(next)
        sectionRefs.current[next]?.focus()
      } else if (e.key === 'ArrowUp') {
        const focused = sectionRefs.current.some((r) => r === document.activeElement)
        if (!focused) return
        e.preventDefault()
        const prev = Math.max(focusedSectionIndex - 1, 0)
        setFocusedSectionIndex(prev)
        sectionRefs.current[prev]?.focus()
      }
    },
    [focusedSectionIndex, sections]
  )

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('keydown', handleSectionKeyDown)
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('keydown', handleSectionKeyDown)
    }
  }, [handleKeyDown, handleSectionKeyDown])

  // Reset refs array sizes when task list changes
  useEffect(() => {
    taskRefs.current = taskRefs.current.slice(0, filteredTasks.length)
    setFocusedTaskIndex(-1)
  }, [filteredTasks.length])

  const TABS: { id: Tab; label: string }[] = [
    { id: 'active', label: 'Active' },
    { id: 'done', label: 'Done' },
    { id: 'archive', label: 'Archive' }
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
                  onClick={() => selectWorkspace(undefined)}
                  className={`w-full rounded-lg px-2 py-2 text-left text-sm transition-colors ${selectedWorkspaceId === undefined ? 'bg-accent/10 font-medium text-accent' : 'text-subtle hover:bg-surface'}`}
                >
                  All workspaces
                </button>
              </li>
              {workspaces.map((ws) => (
                <li key={ws.id}>
                  <button
                    type="button"
                    onClick={() => selectWorkspace(ws.id)}
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
                  ref={(el) => {
                    sectionRefs.current[0] = el
                  }}
                  type="button"
                  onClick={() => setSelectedSectionId(undefined)}
                  className={`w-full rounded-lg px-2 py-2 text-left text-sm transition-colors ${selectedSectionId === undefined ? 'bg-accent/10 font-medium text-accent' : 'text-subtle hover:bg-surface'}`}
                >
                  All sections
                </button>
              </li>
              {sections.map((sec, i) => (
                <li key={sec.id}>
                  <button
                    ref={(el) => {
                      sectionRefs.current[i + 1] = el
                    }}
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

        {/* Search — always visible */}
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

        {/* Task list */}
        {isLoading && <p className="py-8 text-center text-sm text-muted">Loading…</p>}
        {error && <p className="py-8 text-center text-sm text-danger">Failed to load tasks.</p>}
        {!isLoading && !error && filteredTasks.length === 0 && (
          <p className="py-8 text-center text-sm text-muted">No tasks.</p>
        )}
        {!isLoading && !error && (
          <div role="list" className="space-y-2">
            {filteredTasks.map((task, i) => (
              <Link
                key={task.id}
                href={`/todos/${task.id}`}
                ref={(el) => {
                  taskRefs.current[i] = el
                }}
                onFocus={() => setFocusedTaskIndex(i)}
              >
                <TaskCard task={task} />
              </Link>
            ))}
          </div>
        )}

        {/* Quick add bar (includes /, @, # shortcuts) */}
        {activeTab === 'active' && (
          <div className="mt-6">
            <QuickAddBar
              sections={sections}
              labelPresets={labelPresets}
              sectionId={selectedSectionId}
              onAdded={() => mutate()}
            />
          </div>
        )}
      </div>
    </div>
  )
}
