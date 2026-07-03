'use client'

import { useState, useEffect, useRef, useCallback } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { useTodos } from '@/hooks/useTodos'
import { useTodoSettings } from '@/hooks/useTodoSettings'
import { TaskCard } from '@/components/todos/TaskCard'
import { PoliciesBanner } from '@/components/todos/PoliciesBanner'
import QuickAddBar, { type QuickAddBarHandle } from '@/components/todos/QuickAddBar'
import ShortcutHints from '@/components/todos/ShortcutHints'
import TodosSidebar from '@/components/todos/TodosSidebar'
import { Input } from '@/components/ui/input'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'

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
  const [showQuickAdd, setShowQuickAdd] = useState(false)

  const taskRefs = useRef<Array<HTMLDivElement | null>>([])
  const sectionRefs = useRef<Array<HTMLButtonElement | null>>([])
  const quickAddRef = useRef<QuickAddBarHandle>(null)

  const status = activeTab === 'done' ? 'done' : activeTab === 'archive' ? 'archived' : 'open'

  const { data, error, isLoading, mutate } = useTodos({
    workspaceId: selectedWorkspaceId,
    sectionId: selectedSectionId,
    status: activeTab === 'active' ? undefined : status
  })

  const { data: settings, mutate: mutateSettings } = useTodoSettings()

  const tasks = data?.tasks ?? []
  const sections = settings?.sections ?? []
  const workspaces = settings?.workspaces ?? []
  const policies = settings?.policies ?? []
  const labelPresets = settings?.labelPresets ?? []
  const hideShortcutHints = settings?.userSettings?.hideShortcutHints ?? false

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

  const openObsidian = useCallback((taskTitle: string, taskLabels: string[]) => {
    const vault = localStorage.getItem('todos:obsidian_vault')
    if (!vault) return
    const label = taskLabels[0] ?? ''
    const filePath = label ? `${label}/${taskTitle}` : taskTitle
    window.location.href = `obsidian://open?vault=${encodeURIComponent(vault)}&file=${encodeURIComponent(filePath)}`
  }, [])

  async function dismissShortcutHints() {
    const client = createServiceClient(SettingsService)
    await client.updateHideShortcutHints({ hide: true })
    mutateSettings()
  }

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      const target = e.target
      const inInput = target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement

      if (e.key === '/') {
        if (inInput) return
        e.preventDefault()
        setShowQuickAdd(true)
        // focus happens after render via useEffect below
        return
      }

      if (inInput) return

      if (e.key === 's') {
        e.preventDefault()
        setFocusedSectionIndex(0)
        sectionRefs.current[0]?.focus()
        return
      }

      const sectionFocused = sectionRefs.current.some((r) => r === document.activeElement)

      if (sectionFocused) {
        const total = sections.length + 1 // +1 for "All sections"
        if (e.key === 'ArrowDown') {
          e.preventDefault()
          const next = Math.min(focusedSectionIndex + 1, total - 1)
          setFocusedSectionIndex(next)
          sectionRefs.current[next]?.focus()
        } else if (e.key === 'ArrowUp') {
          e.preventDefault()
          const prev = Math.max(focusedSectionIndex - 1, 0)
          setFocusedSectionIndex(prev)
          sectionRefs.current[prev]?.focus()
        } else if (e.key === 'Escape' || e.key === 'ArrowRight') {
          e.preventDefault()
          setFocusedSectionIndex(-1)
          const firstTask = taskRefs.current[0]
          if (firstTask) {
            setFocusedTaskIndex(0)
            firstTask.focus()
          } else if (document.activeElement instanceof HTMLElement) {
            document.activeElement.blur()
          }
        }
        return
      }

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
      } else if (e.key === 'Enter' && focusedTaskIndex >= 0) {
        const task = filteredTasks[focusedTaskIndex]
        if (task) openObsidian(task.title, task.labels)
      }
    },
    [focusedTaskIndex, focusedSectionIndex, filteredTasks, sections.length, openObsidian]
  )

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [handleKeyDown])

  // Focus QuickAddBar after it mounts
  useEffect(() => {
    if (showQuickAdd) {
      quickAddRef.current?.focus()
    }
  }, [showQuickAdd])

  // Reset task refs when list changes
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
    <div className="flex flex-col gap-4">
      {/* Policies banner — above everything */}
      {policies.length > 0 && <PoliciesBanner policies={policies} />}

      {/* Shortcut hints */}
      {!hideShortcutHints && <ShortcutHints onDismiss={dismissShortcutHints} />}

      <div className="flex gap-6">
        <TodosSidebar
          workspaces={workspaces}
          sections={sections}
          selectedWorkspaceId={selectedWorkspaceId}
          selectedSectionId={selectedSectionId}
          onSelectWorkspace={selectWorkspace}
          onSelectSection={setSelectedSectionId}
          sectionRefs={sectionRefs}
        />

        {/* Main content */}
        <div className="min-w-0 flex-1">
          {/* Search — always visible */}
          <div className="mb-4">
            <Input
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search tasks…"
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

          {/* Quick add bar — shown on / */}
          {activeTab === 'active' && showQuickAdd && (
            <div className="mb-4">
              <QuickAddBar
                ref={quickAddRef}
                sections={sections}
                labelPresets={labelPresets}
                sectionId={selectedSectionId}
                onAdded={() => mutate()}
                onClose={() => setShowQuickAdd(false)}
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
              {filteredTasks.map((task, i) => (
                <div
                  key={task.id}
                  ref={(el) => {
                    taskRefs.current[i] = el
                  }}
                  tabIndex={0}
                  onFocus={() => setFocusedTaskIndex(i)}
                >
                  <TaskCard
                    task={task}
                    onClick={() => openObsidian(task.title, task.labels)}
                    onChanged={() => mutate()}
                  />
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
