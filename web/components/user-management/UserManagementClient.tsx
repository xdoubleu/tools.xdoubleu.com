'use client'

import { mutate } from 'swr'
import { useUsers, useSetRole, useSetAppAccess } from '@/hooks/useUserManagement'
import type { AppUser } from '@/lib/gen/access/v1/access_pb'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { swrKeys } from '@/lib/swrKeys'

const APP_NAMES = ['games', 'books', 'feeds', 'mealplans', 'recipes', 'shoppinglist', 'watchparty']

function UserRow({ user }: { user: AppUser }) {
  const setRole = useSetRole()
  const setAppAccess = useSetAppAccess()

  async function handleRoleChange(role: string) {
    await setRole(user.id, role)
    await mutate(swrKeys.userManagementUsers)
  }

  async function handleAccessToggle(appName: string) {
    const grant = !user.appAccess.includes(appName)
    await setAppAccess(user.id, appName, grant)
    await mutate(swrKeys.userManagementUsers)
  }

  return (
    <TableRow>
      <TableCell className="break-all text-fg">{user.email}</TableCell>
      <TableCell>
        <Select
          aria-label={`Role for ${user.email}`}
          value={user.role}
          onChange={(e) => handleRoleChange(e.target.value)}
          className="w-auto"
        >
          <option value="user">user</option>
          <option value="admin">admin</option>
        </Select>
      </TableCell>
      {APP_NAMES.map((appName) => {
        const hasAccess = user.appAccess.includes(appName)
        return (
          <TableCell key={appName} className="text-center">
            <Button
              size="sm"
              variant={hasAccess ? 'default' : 'secondary'}
              onClick={() => handleAccessToggle(appName)}
            >
              {hasAccess ? 'Revoke' : 'Grant'}
            </Button>
          </TableCell>
        )
      })}
    </TableRow>
  )
}

export default function UserManagementClient() {
  const { data, isLoading, error } = useUsers()
  const users = data?.users ?? []

  if (isLoading) {
    return <LoadingState className="py-16 text-center text-sm" />
  }

  if (error) {
    return <ErrorState what="users" className="py-16 text-center text-sm" />
  }

  return (
    <PageContainer>
      <PageHeader title="User Management" />

      <Table className="text-left">
        <TableHeader>
          <TableRow>
            <TableHead>Email</TableHead>
            <TableHead>Role</TableHead>
            {APP_NAMES.map((name) => (
              <TableHead key={name} className="text-center capitalize">
                {name}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.length === 0 ? (
            <TableRow>
              <TableCell colSpan={2 + APP_NAMES.length} className="py-6 text-center text-muted">
                No users found.
              </TableCell>
            </TableRow>
          ) : (
            users.map((user) => <UserRow key={user.id} user={user} />)
          )}
        </TableBody>
      </Table>
    </PageContainer>
  )
}
