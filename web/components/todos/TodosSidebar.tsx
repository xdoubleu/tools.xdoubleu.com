'use client'

import Link from 'next/link'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'
import { cn } from '@/lib/cn'
import type { Section, Workspace } from '@/lib/gen/todos/v1/settings_pb'

const navButtonClass = (active: boolean) =>
  cn(
    'h-auto w-full justify-start rounded-lg px-2 py-2 text-sm font-normal',
    active ? 'bg-accent/10 font-medium text-accent hover:bg-accent/10' : 'text-subtle'
  )

interface TodosSidebarProps {
  workspaces: Workspace[]
  sections: Section[]
  selectedWorkspaceId: string | undefined
  selectedSectionId: string | undefined
  onSelectWorkspace: (id: string | undefined) => void
  onSelectSection: (id: string | undefined) => void
  /** Shared with the page's keyboard navigation ("s" jumps to sections). */
  sectionRefs: React.RefObject<Array<HTMLButtonElement | null>>
}

export default function TodosSidebar({
  workspaces,
  sections,
  selectedWorkspaceId,
  selectedSectionId,
  onSelectWorkspace,
  onSelectSection,
  sectionRefs
}: TodosSidebarProps) {
  return (
    <aside className="w-full shrink-0 md:w-48">
      {/* Workspaces */}
      {workspaces.length > 0 && (
        <section className="mb-4">
          <h2 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted">
            Workspaces
          </h2>
          <ul>
            <li>
              <Button
                type="button"
                variant="ghost"
                onClick={() => onSelectWorkspace(undefined)}
                className={navButtonClass(selectedWorkspaceId === undefined)}
              >
                All workspaces
              </Button>
            </li>
            {workspaces.map((ws) => (
              <li key={ws.id}>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => onSelectWorkspace(ws.id)}
                  className={navButtonClass(selectedWorkspaceId === ws.id)}
                >
                  {ws.name}
                </Button>
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
              <Button
                ref={(el) => {
                  sectionRefs.current[0] = el
                }}
                type="button"
                variant="ghost"
                onClick={() => onSelectSection(undefined)}
                className={navButtonClass(selectedSectionId === undefined)}
              >
                All sections
              </Button>
            </li>
            {sections.map((sec, i) => (
              <li key={sec.id}>
                <Button
                  ref={(el) => {
                    sectionRefs.current[i + 1] = el
                  }}
                  type="button"
                  variant="ghost"
                  onClick={() => onSelectSection(sec.id)}
                  className={navButtonClass(selectedSectionId === sec.id)}
                >
                  {sec.name}
                </Button>
              </li>
            ))}
          </ul>
        </section>
      )}
      {/* Settings link */}
      <div className="mt-6 border-t border-border pt-4">
        <Button
          asChild
          variant="ghost"
          className="h-auto w-full justify-start gap-2 rounded-lg px-2 py-2 text-sm font-normal text-subtle hover:text-fg"
        >
          <Link href="/todos/settings">
            <SettingsIcon />
            Settings
          </Link>
        </Button>
      </div>
    </aside>
  )
}
