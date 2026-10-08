'use client'

import { mutate } from 'swr'
import { useUsers, useSetRole, useSetAppAccess } from '@/hooks/useUserManagement'
import type { AppUser } from '@/lib/gen/access/v1/access_pb'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { Card } from '@/components/ui/card'
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

const APP_NAMES = [
  'games',
  'books',
  'feeds',
  'mealplans',
  'movies',
  'podcasts',
  'recipes',
  'shoppinglist',
  'watchparty'
]

function useUserActions(user: AppUser) {
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

  return { handleRoleChange, handleAccessToggle }
}

/** Role select. Shared by the mobile card and the desktop table row. */
function RoleSelect({ user }: { user: AppUser }) {
  const { handleRoleChange } = useUserActions(user)
  return (
    <Select
      aria-label={`Role for ${user.email}`}
      value={user.role}
      onChange={(e) => void handleRoleChange(e.target.value)}
      className="w-auto"
    >
      <option value="user">user</option>
      <option value="admin">admin</option>
    </Select>
  )
}

/** One app's Grant/Revoke button. Shared by the mobile card and the table row. */
function AppAccessButton({ user, appName }: { user: AppUser; appName: string }) {
  const { handleAccessToggle } = useUserActions(user)
  const hasAccess = user.appAccess.includes(appName)
  return (
    <Button
      size="sm"
      variant={hasAccess ? 'default' : 'secondary'}
      onClick={() => void handleAccessToggle(appName)}
    >
      {hasAccess ? 'Revoke' : 'Grant'}
    </Button>
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

      {/* Mobile-first card-per-row view; the 9-column table would force a
          wide sideways scroll below `sm`. */}
      <div className="space-y-3 sm:hidden">
        {users.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted">No users found.</p>
        ) : (
          users.map((user) => (
            <Card key={user.id} className="space-y-3 p-4">
              <p className="break-all text-sm font-medium text-fg">{user.email}</p>
              <div className="flex flex-wrap items-center gap-2">
                <RoleSelect user={user} />
                {APP_NAMES.map((appName) => (
                  <AppAccessButton key={appName} user={user} appName={appName} />
                ))}
              </div>
            </Card>
          ))
        )}
      </div>

      <div className="hidden sm:block">
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
              users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="break-all text-fg">{user.email}</TableCell>
                  <TableCell>
                    <RoleSelect user={user} />
                  </TableCell>
                  {APP_NAMES.map((appName) => (
                    <TableCell key={appName} className="text-center">
                      <AppAccessButton user={user} appName={appName} />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </PageContainer>
  )
}
