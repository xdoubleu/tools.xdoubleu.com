import UserManagementClient from '@/components/user-management/UserManagementClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { AccessService } from '@/lib/gen/access/v1/access_pb'

export default async function UserManagementPage() {
  const client = await createServerClient(AccessService)
  const users = await fetchOrNull(() => client.listUsers({}))

  return (
    <SWRFallback fallback={users ? { [swrKeys.userManagementUsers]: users } : {}}>
      <UserManagementClient />
    </SWRFallback>
  )
}
