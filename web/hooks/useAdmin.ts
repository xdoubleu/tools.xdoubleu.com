import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { AdminService } from '@/lib/gen/admin/v1/admin_connect'
import type { ListUsersResponse } from '@/lib/gen/admin/v1/admin_pb'

export function useUsers() {
  const client = createServiceClient(AdminService)
  return useSWR<ListUsersResponse, Error>('/admin/users', () =>
    client.listUsers({})
  )
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
