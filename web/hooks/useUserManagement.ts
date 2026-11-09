import useSWR, { mutate } from 'swr'
import { createServiceClient } from '@/lib/client'
import { AccessService } from '@/lib/gen/access/v1/access_pb'
import { AgentAccountService } from '@/lib/gen/factory/v1/agent_accounts_pb'
import type { ListAgentAccountsResponse } from '@/lib/gen/factory/v1/agent_accounts_pb'
import type { ListUsersResponse } from '@/lib/gen/access/v1/access_pb'
import { swrKeys } from '@/lib/swrKeys'

export type AgentLevel = 'read' | 'write' | 'none'

export function useUsers() {
  const client = createServiceClient(AccessService)
  return useSWR<ListUsersResponse, Error>(swrKeys.userManagementUsers, () => client.listUsers({}))
}

export function useSetRole() {
  const client = createServiceClient(AccessService)
  return (userId: string, role: string) => client.setRole({ userId, role })
}

export function useSetAppAccess() {
  const client = createServiceClient(AccessService)
  return (userId: string, appName: string, grant: boolean) =>
    client.setAppAccess({ userId, appName, grant })
}

export function useAgents() {
  const client = createServiceClient(AgentAccountService)
  return useSWR<ListAgentAccountsResponse, Error>(swrKeys.userManagementAgents, () =>
    client.listAgentAccounts({})
  )
}

/** Agent writes; each revalidates the agent list. Secrets come back once, in the result. */
export function useAgentActions() {
  const agents = createServiceClient(AgentAccountService)
  const access = createServiceClient(AccessService)
  const refresh = () => mutate(swrKeys.userManagementAgents)

  return {
    create: async (name: string, totpCode: string) => {
      const created = await agents.createAgentAccount({ name, totpCode })
      await refresh()
      return created
    },
    revoke: async (id: string) => {
      await agents.revokeAgentAccount({ id })
      await refresh()
    },
    rotate: async (id: string, totpCode: string) => {
      const rotated = await agents.rotateAgentAccountSecret({ id, totpCode })
      await refresh()
      return rotated
    },
    joinFamily: async (id: string, totpCode: string) => {
      await agents.addAgentAccountToFamily({ id, totpCode })
      await refresh()
    },
    leaveFamily: async (id: string) => {
      await agents.removeAgentAccountFromFamily({ id })
      await refresh()
    },
    /** level is "read", "write" or "none". */
    setAccess: async (userId: string, appName: string, level: AgentLevel) => {
      await access.setAgentAppAccess({ userId, appName, level })
      await refresh()
    }
  }
}
