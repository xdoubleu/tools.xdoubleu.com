'use client'

import { useTodoSettings } from '@/hooks/useTodoSettings'

export default function TodoSettingsPage() {
  const { data, isLoading, error } = useTodoSettings()

  if (isLoading) {
    return <p className="py-16 text-center text-sm text-muted">Loading…</p>
  }

  if (error || !data) {
    return <p className="py-16 text-center text-sm text-red-500">Failed to load settings.</p>
  }

  const { sections, labelPresets, urlPatterns, archive, policies, workspaces } = data

  return (
    <div className="mx-auto max-w-2xl space-y-10">
      <h1 className="text-xl font-semibold text-fg">Settings</h1>

      {/* Workspaces */}
      <section aria-labelledby="workspaces-heading">
        <h2
          id="workspaces-heading"
          className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
        >
          Workspaces
        </h2>
        {workspaces.length === 0 ? (
          <p className="text-sm text-muted">No workspaces.</p>
        ) : (
          <ul className="space-y-1">
            {workspaces.map((ws) => (
              <li
                key={ws.id}
                className="rounded border border-border bg-card px-3 py-2 text-sm text-subtle"
              >
                {ws.name}
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* Sections */}
      <section aria-labelledby="sections-heading">
        <h2
          id="sections-heading"
          className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
        >
          Sections
        </h2>
        {sections.length === 0 ? (
          <p className="text-sm text-muted">No sections.</p>
        ) : (
          <ul className="space-y-1">
            {sections.map((sec) => (
              <li
                key={sec.id}
                className="rounded border border-border bg-card px-3 py-2 text-sm text-subtle"
              >
                {sec.name}
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* Label Presets */}
      <section aria-labelledby="labels-heading">
        <h2
          id="labels-heading"
          className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
        >
          Label Presets
        </h2>
        {labelPresets.length === 0 ? (
          <p className="text-sm text-muted">No label presets.</p>
        ) : (
          <ul className="flex flex-wrap gap-2">
            {labelPresets.map((preset) => (
              <li key={preset.value}>
                <span
                  className="rounded px-2 py-1 text-xs font-medium"
                  style={{
                    backgroundColor: `${preset.color}22`,
                    color: preset.color,
                    border: `1px solid ${preset.color}44`
                  }}
                >
                  {preset.value}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* URL Patterns */}
      <section aria-labelledby="patterns-heading">
        <h2
          id="patterns-heading"
          className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
        >
          URL Patterns
        </h2>
        {urlPatterns.length === 0 ? (
          <p className="text-sm text-muted">No URL patterns.</p>
        ) : (
          <ul className="space-y-2">
            {urlPatterns.map((pattern) => (
              <li key={pattern.id} className="rounded border border-border bg-card p-3">
                <p className="text-sm font-medium text-subtle">{pattern.platformName}</p>
                <p className="text-xs text-muted">{pattern.urlPrefix}</p>
                {pattern.label && (
                  <span className="mt-1 inline-block rounded bg-surface px-1.5 py-0.5 text-xs text-muted">
                    {pattern.label}
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* Archive Settings */}
      <section aria-labelledby="archive-heading">
        <h2
          id="archive-heading"
          className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
        >
          Archive Settings
        </h2>
        <p className="text-sm text-subtle">
          Tasks are archived after <strong>{archive?.archiveAfterHours ?? 0}</strong> hour
          {(archive?.archiveAfterHours ?? 0) === 1 ? '' : 's'} once completed.
        </p>
      </section>

      {/* Policies */}
      <section aria-labelledby="policies-heading">
        <h2
          id="policies-heading"
          className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
        >
          Policies
        </h2>
        {policies.length === 0 ? (
          <p className="text-sm text-muted">No policies.</p>
        ) : (
          <ul className="space-y-2">
            {policies.map((policy) => (
              <li
                key={policy.id}
                className="rounded border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-200"
              >
                {policy.text}
                <p className="mt-1 text-xs text-amber-600 dark:text-amber-400">
                  Re-appears after {policy.reappearAfterHours}h
                </p>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
