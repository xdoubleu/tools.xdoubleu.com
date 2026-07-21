'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'

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
    "Read-only access to this app's own domain data (games, reading, recipes, meal plans, shopping list, todos), gated by your own per-app access.",
  command: 'claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp'
}

const MONITORING_MCP: McpCommand = {
  key: 'monitoring',
  title: 'Monitoring MCP',
  description: 'Read-only admin observability signals (jobs, usage, storage, database, deploys).',
  command: 'claude mcp add --transport http tools-obs https://tools.xdoubleu.com/api/monitoring/mcp'
}

export function McpSetupSection({ role }: { role: string }) {
  const [copiedKey, setCopiedKey] = useState('')

  const handleCopy = async (cmd: McpCommand) => {
    await navigator.clipboard.writeText(cmd.command)
    setCopiedKey(cmd.key)
    setTimeout(() => setCopiedKey(''), 2000)
  }

  const commands = role === 'admin' ? [APPS_MCP, MONITORING_MCP] : [APPS_MCP]

  return (
    <section>
      <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-muted">MCP Setup</h2>
      <p className="mb-4 text-sm text-subtle">
        Connect a local Claude Code CLI to this app&apos;s read-only MCP servers. Running the
        command opens a browser consent screen the first time — OAuth is handled automatically.
      </p>

      <div className="space-y-4">
        {commands.map((cmd) => (
          <div key={cmd.key}>
            <p className="mb-1 text-sm font-medium text-fg">{cmd.title}</p>
            <p className="mb-2 text-sm text-subtle">{cmd.description}</p>
            <div className="flex items-center gap-2">
              <code className="flex-1 overflow-x-auto rounded-lg border border-border bg-surface px-3 py-2 font-mono text-xs">
                {cmd.command}
              </code>
              <Button variant="secondary" size="sm" onClick={() => handleCopy(cmd)}>
                {copiedKey === cmd.key ? 'Copied!' : 'Copy'}
              </Button>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}
