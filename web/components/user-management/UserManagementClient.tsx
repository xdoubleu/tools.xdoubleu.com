import { mutate } from 'swr'
import { useUsers, useSetRole, useSetAppAccess } from '@/hooks/useUserManagement'
import type { AppUser } from '@/lib/gen/access/v1/access_pb'
import { useState } from 'react'
import { Checkbox } from '@/components/ui/checkbox'
import { ConfirmDialog } from '@/components/ui/dialog'
import { Select } from '@/components/ui/select'
import { Card } from '@/components/ui/card'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderLink } from '@/components/ui/page-header'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { TogglePill } from '@/components/ui/toggle-pill'
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
  'taste',
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

/** Grants at once; a revoke asks first. Returns the state and the dialog to render. */
function useAccessChange(user: AppUser, appName: string) {
  const { handleAccessToggle } = useUserActions(user)
  const [confirming, setConfirming] = useState(false)
  const hasAccess = user.appAccess.includes(appName)
  const dialog = (
    <ConfirmDialog
      open={confirming}
      onOpenChange={setConfirming}
      title={`Revoke ${appName} from ${user.email}?`}
      description="They lose access to the app straight away."
      confirmLabel="Revoke"
      destructive
      onConfirm={() => void handleAccessToggle(appName)}
    />
  )
  const change = () => (hasAccess ? setConfirming(true) : void handleAccessToggle(appName))
  return { hasAccess, change, dialog }
}

/** One app's access as a checkbox that shows the current state. Used in the table row. */
/** The user's full name over their email; the email alone when no name is set. */
function UserIdentity({ user }: { user: AppUser }) {
  return (
    <>
      {user.displayName && <p className="text-sm font-medium text-fg">{user.displayName}</p>}
      <p className={user.displayName ? 'text-xs text-muted' : 'text-sm font-medium text-fg'}>
        {user.email}
      </p>
    </>
  )
}

function AppAccessCheckbox({ user, appName }: { user: AppUser; appName: string }) {
  const { hasAccess, change, dialog } = useAccessChange(user, appName)
  return (
    <>
      <Checkbox
        checked={hasAccess}
        onChange={change}
        label={<span className="sr-only">{`${appName} access for ${user.email}`}</span>}
        labelClassName="justify-center"
      />
      {dialog}
    </>
  )
}

/** One app's access as a pill named after the app, for the mobile card where no column header names it. */
function AppAccessPill({ user, appName }: { user: AppUser; appName: string }) {
  const { hasAccess, change, dialog } = useAccessChange(user, appName)
  return (
    <>
      <TogglePill label={appName} active={hasAccess} onClick={change} />
      {dialog}
    </>
  )
}

export default function UserManagementClient() {
  const { data, isLoading, error } = useUsers()
  const users = data?.users ?? []

  if (isLoading) {
    return <LoadingState placement="page" />
  }

  if (error) {
    return <ErrorState what="users" placement="page" />
  }

  return (
    <PageContainer size="wide">
      <PageHeader
        title="User management"
        actions={<PageHeaderLink href="/user-management/agents">Agents</PageHeaderLink>}
      />

      {/* Mobile-first card-per-row view; the 9-column table would force a
          wide sideways scroll below `sm`. */}
      <div className="space-y-3 sm:hidden">
        {users.length === 0 ? (
          <EmptyState>No users found.</EmptyState>
        ) : (
          users.map((user) => (
            <Card key={user.id} className="space-y-3">
              <UserIdentity user={user} />
              <div className="flex flex-wrap items-center gap-2">
                <RoleSelect user={user} />
              </div>
              <div
                role="group"
                className="flex flex-wrap gap-2"
                aria-label={`App access for ${user.email}`}
              >
                {APP_NAMES.map((appName) => (
                  <AppAccessPill key={appName} user={user} appName={appName} />
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
              <TableHead>User</TableHead>
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
                  <TableCell className="break-all text-fg">
                    <UserIdentity user={user} />
                  </TableCell>
                  <TableCell>
                    <RoleSelect user={user} />
                  </TableCell>
                  {APP_NAMES.map((appName) => (
                    <TableCell key={appName} className="text-center">
                      <AppAccessCheckbox user={user} appName={appName} />
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
