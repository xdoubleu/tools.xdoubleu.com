import { useState } from 'react'
import { Harness } from '@/lib/gen/factory/v1/factory_pb'
import type { Chat } from '@/lib/gen/factory/v1/factory_pb'
import { repoLabel, reposOf } from '@/lib/factory/board'
import { stateLabel } from '@/lib/factory/chat'
import {
  useChatActions,
  useFactoryChats,
  useFactoryRepos,
  useFactoryRunners
} from '@/hooks/useFactory'
import { useRouter, useSearchParams } from '@/hooks/navigation'
import ChatView from '@/components/factory/ChatView'
import AppSections from '@/components/AppSections'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { GroupedList, GroupedListItem } from '@/components/ui/grouped-list'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { SectionHeading } from '@/components/ui/section-heading'
import { Select } from '@/components/ui/select'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

const HARNESSES = [
  { value: Harness.OPENCODE, label: 'OpenCode' },
  { value: Harness.CLAUDE_CODE, label: 'Claude Code' }
]

/** Starts a chat on a registered repo, on the first runner that is online. */
function NewChat({ onStarted }: { onStarted: (chat: Chat) => void }) {
  const { data: repos } = useFactoryRepos()
  const { data: runners } = useFactoryRunners()
  const { start } = useChatActions()
  const [repoId, setRepoId] = useState('')
  const [harness, setHarness] = useState(Harness.OPENCODE)
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)

  const list = reposOf(repos)
  const selected = repoId || list[0]?.id || ''
  const runner = runners?.runners.find((r) => r.online && r.revokedAt === '')

  const begin = async () => {
    setPending(true)
    setFailed(false)
    try {
      const chat = await start(selected, harness, runner?.id ?? '')
      if (chat) onStarted(chat)
    } catch {
      setFailed(true)
    }
    setPending(false)
  }

  return (
    <Card variant="inset" padding="md" className="mb-6 space-y-4">
      <SectionHeading>New chat</SectionHeading>
      <Field label="Repo" htmlFor="chat-repo">
        <Select id="chat-repo" value={selected} onChange={(e) => setRepoId(e.target.value)}>
          {list.map((repo) => (
            <option key={repo.id} value={repo.id}>
              {repoLabel(repo)}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Harness" htmlFor="chat-harness">
        <Select
          id="chat-harness"
          value={harness}
          onChange={(e) => setHarness(Number(e.target.value))}
        >
          {HARNESSES.map((h) => (
            <option key={h.value} value={h.value}>
              {h.label}
            </option>
          ))}
        </Select>
      </Field>
      {runners && !runner && <Alert tone="warn">No runner is online to host a chat.</Alert>}
      {failed && (
        <Alert tone="danger">Couldn&apos;t start the chat. Check the runner and try again.</Alert>
      )}
      <Button onClick={() => void begin()} disabled={pending || !selected || !runner}>
        {pending ? 'Starting…' : 'Start chat'}
      </Button>
    </Card>
  )
}

function harnessLabel(h: Harness): string {
  return HARNESSES.find((x) => x.value === h)?.label ?? 'Harness'
}

/** The factory chat page: pick a chat, or start one. */
export default function ChatClient() {
  const params = useSearchParams()
  const router = useRouter()
  const { data, error, isLoading } = useFactoryChats()
  const { data: repos } = useFactoryRepos()
  const id = params.get('chat')
  const [repoFilter, setRepoFilter] = useState('')
  const open = (chatId: string) => router.push(`/factory/chat?chat=${encodeURIComponent(chatId)}`)

  if (id) {
    return (
      <PageContainer size="narrow">
        <ChatView
          key={id}
          id={id}
          chat={data?.chats.find((c) => c.id === id)}
          onLeave={() => router.push('/factory/chat')}
        />
      </PageContainer>
    )
  }

  const repoName = (repoId: string) => {
    const repo = reposOf(repos).find((r) => r.id === repoId)
    return repo ? repoLabel(repo) : 'Repo'
  }

  const chats = data?.chats.filter((c) => !repoFilter || c.repoId === repoFilter) ?? []

  return (
    <PageContainer size="narrow">
      <PageHeader title="Chat" />
      <AppSections app="factory" />
      <NewChat onStarted={(chat) => open(chat.id)} />
      {error && !data && <ErrorState what="chats" />}
      {isLoading && !data && <LoadingState label="chats" />}
      {data && data.chats.length > 0 && (
        <Field label="Filter by repo" htmlFor="chat-filter" className="mb-4">
          <Select
            id="chat-filter"
            value={repoFilter}
            onChange={(e) => setRepoFilter(e.target.value)}
          >
            <option value="">All repos</option>
            {reposOf(repos).map((repo) => (
              <option key={repo.id} value={repo.id}>
                {repoLabel(repo)}
              </option>
            ))}
          </Select>
        </Field>
      )}
      {data && data.chats.length === 0 && <EmptyState>No chats yet.</EmptyState>}
      {data && data.chats.length > 0 && chats.length === 0 && (
        <EmptyState>No chats for this repo.</EmptyState>
      )}
      {chats.length > 0 && (
        <GroupedList aria-label="Chats">
          {chats.map((chat) => (
            <GroupedListItem key={chat.id}>
              <div className="min-w-0 flex-1">
                <p className="break-words font-medium">{repoName(chat.repoId)}</p>
                <p className="text-xs text-muted">
                  {harnessLabel(chat.harness)}
                  {chat.taskId && ' · linked to a task'}
                </p>
                {chat.lastMessage && (
                  <p className="truncate text-sm text-muted">{chat.lastMessage}</p>
                )}
              </div>
              <Badge
                variant={chat.state === 'stopped' || chat.state === 'paused' ? 'secondary' : 'success'}
              >
                {stateLabel(chat.state)}
              </Badge>
              <Button variant="secondary" onClick={() => open(chat.id)}>
                {chat.state === 'stopped' ? 'View' : 'Resume'}
              </Button>
            </GroupedListItem>
          ))}
        </GroupedList>
      )}
    </PageContainer>
  )
}
