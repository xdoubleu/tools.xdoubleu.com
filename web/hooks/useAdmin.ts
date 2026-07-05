import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { AdminService } from '@/lib/gen/admin/v1/admin_pb'
import type { ListUsersResponse } from '@/lib/gen/admin/v1/admin_pb'
import { swrKeys } from '@/lib/swrKeys'

export function useUsers() {
  const client = createServiceClient(AdminService)
  return useSWR<ListUsersResponse, Error>(swrKeys.adminUsers, () => client.listUsers({}))
}

export function useSetRole() {
  const client = createServiceClient(AdminService)
  return (userId: string, role: string) => client.setRole({ userId, role })
}

export function useSetAppAccess() {
  const client = createServiceClient(AdminService)
  return (userId: string, appName: string, grant: boolean) =>
    client.setAppAccess({ userId, appName, grant })
}
