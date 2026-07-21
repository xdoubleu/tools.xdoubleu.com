import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { AccessService } from '@/lib/gen/access/v1/access_pb'
import type { ListUsersResponse } from '@/lib/gen/access/v1/access_pb'
import { swrKeys } from '@/lib/swrKeys'

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
