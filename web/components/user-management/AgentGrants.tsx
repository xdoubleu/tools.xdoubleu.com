import type { AgentLevel } from '@/hooks/useUserManagement'
import { Field } from '@/components/ui/field'
import { Select } from '@/components/ui/select'

export const AGENT_APPS = [
  'factory',
  'books',
  'feeds',
  'games',
  'learningpaths',
  'mealplans',
  'movies',
  'podcasts',
  'recipes',
  'shoppinglist',
  'taste',
  'trains'
]

const LEVELS: AgentLevel[] = ['none', 'read', 'write']

/** A none/read/write select per app an agent can be granted. */
export function AgentGrants({
  name,
  idPrefix,
  levelOf,
  onChange
}: {
  name: string
  idPrefix: string
  levelOf: (app: string) => AgentLevel
  onChange: (app: string, level: AgentLevel) => void
}) {
  return (
    <div
      role="group"
      aria-label={`App access for ${name}`}
      className="grid grid-cols-2 gap-2 sm:grid-cols-3"
    >
      {AGENT_APPS.map((app) => (
        <Field key={app} label={app} htmlFor={`${idPrefix}-${app}`} className="capitalize">
          <Select
            id={`${idPrefix}-${app}`}
            aria-label={`${app} access for ${name}`}
            value={levelOf(app)}
            onChange={(e) => {
              const level = LEVELS.find((l) => l === e.target.value)
              if (level) onChange(app, level)
            }}
          >
            {LEVELS.map((level) => (
              <option key={level} value={level}>
                {level}
              </option>
            ))}
          </Select>
        </Field>
      ))}
    </div>
  )
}
