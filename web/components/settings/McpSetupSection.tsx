import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { SettingsSection } from '@/components/ui/settings-section'
import { useCopyFeedback } from '@/hooks/useCopyFeedback'

type McpCommand = {
  key: string
  title: string
  description: string
  command: string
}

const APPS_MCP: McpCommand = {
  key: 'apps',
  title: 'Apps MCP',
  description:
    "Read-only access to this app's own domain data (games, books, recipes, meal plans, shopping list), gated by your own per-app access.",
  command: 'claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp'
}

export function McpSetupSection() {
  const { copy, copiedKey } = useCopyFeedback()

  const handleCopy = (cmd: McpCommand) => copy(cmd.command, cmd.key)

  const commands = [APPS_MCP]

  return (
    <SettingsSection
      title="MCP setup"
      description="Connect a local Claude Code CLI to this app's read-only MCP servers. Running the command opens a browser consent screen the first time — OAuth is handled automatically."
    >
      <div className="space-y-4">
        {commands.map((cmd) => (
          <div key={cmd.key}>
            <p className="mb-1 text-sm font-medium text-fg">{cmd.title}</p>
            <p className="mb-2 text-sm text-subtle">{cmd.description}</p>
            <div className="flex items-center gap-2">
              <Card variant="inset" padding="sm" className="min-w-0 flex-1 overflow-x-auto">
                <code className="whitespace-nowrap font-mono text-xs">{cmd.command}</code>
              </Card>
              <Button variant="secondary" size="sm" onClick={() => handleCopy(cmd)}>
                {copiedKey === cmd.key ? 'Copied!' : 'Copy'}
              </Button>
            </div>
          </div>
        ))}
      </div>
    </SettingsSection>
  )
}
