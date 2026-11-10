import type { AgentLevel } from '@/hooks/useUserManagement'
import { useAgents } from '@/hooks/useUserManagement'
import { AgentGrants } from '@/components/user-management/AgentGrants'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select } from '@/components/ui/select'

export type Mode = 'self' | 'new' | 'existing'

export interface AgentChoiceState {
  mode: Mode
  name: string
  grants: Record<string, AgentLevel>
  existingId: string
}

/** A new agent holds factory write, like one made on the agents page. */
export const NEW_AGENT_GRANTS: Record<string, AgentLevel> = { factory: 'write' }

/** Who the client signs in as: the admin, a new agent account or an existing one. */
export default function AgentChoice({
  state,
  onChange
}: {
  state: AgentChoiceState
  onChange: (next: Partial<AgentChoiceState>) => void
}) {
  const { data, error } = useAgents()
  const agents = (data?.accounts ?? []).filter((a) => !a.revokedAt)

  return (
    <div className="space-y-4">
      <RadioGroup
        name="consent-subject"
        aria-label="Sign in as"
        value={state.mode}
        onChange={(mode) => {
          if (mode === 'self' || mode === 'new' || mode === 'existing') onChange({ mode })
        }}
      >
        <RadioGroupItem value="self" label="Sign in as me" />
        <RadioGroupItem value="new" label="Delegate to a new agent" />
        <RadioGroupItem
          value="existing"
          label="Delegate to an existing agent"
          disabled={agents.length === 0}
        />
      </RadioGroup>

      {state.mode === 'new' && (
        <div className="space-y-4">
          <Field label="Agent name" htmlFor="consent-agent-name">
            <Input
              id="consent-agent-name"
              value={state.name}
              maxLength={80}
              onChange={(e) => onChange({ name: e.target.value })}
            />
          </Field>
          <AgentGrants
            name="the new agent"
            idPrefix="consent-agent"
            levelOf={(app) => state.grants[app] ?? 'none'}
            onChange={(app, level) => onChange({ grants: { ...state.grants, [app]: level } })}
          />
        </div>
      )}

      {state.mode === 'existing' && (
        <Field
          label="Agent"
          htmlFor="consent-agent-existing"
          hint="Change its app access on the agents page."
        >
          <Select
            id="consent-agent-existing"
            value={state.existingId}
            onChange={(e) => onChange({ existingId: e.target.value })}
          >
            <option value="">Choose an agent</option>
            {agents.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </Select>
        </Field>
      )}
      {error && !data && <p className="text-sm text-muted">Couldn&apos;t load your agents.</p>}
    </div>
  )
}
