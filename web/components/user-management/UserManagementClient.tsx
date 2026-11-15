import { mutate } from 'swr'
import {
  useUsers,
  useSetRole,
  useSetAppAccess,
  useSetDisplayName,
  useCreateUser,
  useDeactivateUser
} from '@/hooks/useUserManagement'
import { Code, ConnectError } from '@connectrpc/connect'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import type { AppUser } from '@/lib/gen/access/v1/access_pb'
import { useState } from 'react'
import { Checkbox } from '@/components/ui/checkbox'
import { ConfirmDialog } from '@/components/ui/dialog'
import { FieldError } from '@/components/ui/field-error'
import { Input } from '@/components/ui/input'
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
  'taste'
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

/** The user's email over a full-name field that saves on blur; a blank name is not saved. */
function UserIdentity({ user }: { user: AppUser }) {
  const setDisplayName = useSetDisplayName()
  const [name, setName] = useState(user.displayName)
  const [error, setError] = useState('')

  async function save() {
    const trimmed = name.trim()
    if (!trimmed || trimmed === user.displayName) {
      setName(user.displayName)
      return
    }
    setError('')
    try {
      await setDisplayName(user.id, trimmed)
      await mutate(swrKeys.userManagementUsers)
    } catch {
      setError('Failed to save full name.')
    }
  }

  return (
    <div className="space-y-1">
      <p className="break-all text-sm font-medium text-fg">{user.email}</p>
      <Input
        aria-label={`Full name for ${user.email}`}
        placeholder="Full name"
        value={name}
        onChange={(e) => setName(e.target.value)}
        onBlur={() => void save()}
        aria-describedby={error ? `name-error-${user.id}` : undefined}
      />
      <FieldError id={`name-error-${user.id}`}>{error}</FieldError>
    </div>
  )
}

/** One app's access as a checkbox that shows the current state. Used in the table row. */
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

/** Why an invite or deactivation failed, in the admin's words. */
function failureText(err: unknown, fallback: string): string {
  const error = ConnectError.from(err)
  if (error.code === Code.AlreadyExists) return 'A user with this email already exists.'
  if (error.code === Code.InvalidArgument) return error.rawMessage
  if (error.code === Code.FailedPrecondition) return 'Inviting people is not set up.'
  return fallback
}

/** Adds a person: the identity provider mails them an invite. */
function InviteForm() {
  const createUser = useCreateUser()
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [invited, setInvited] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setInvited('')
    setBusy(true)
    try {
      const created = await createUser(email.trim(), name.trim())
      setInvited(created.user?.email ?? email)
      setEmail('')
      setName('')
      await mutate(swrKeys.userManagementUsers)
    } catch (err) {
      setError(failureText(err, 'Failed to invite this person.'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card className="space-y-3">
      <form onSubmit={(e) => void submit(e)} className="flex flex-col gap-2 sm:flex-row">
        <Input
          type="email"
          aria-label="Email of the person to invite"
          placeholder="Email"
          autoComplete="off"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
        <Input
          aria-label="Full name of the person to invite"
          placeholder="Full name"
          autoComplete="off"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <Button type="submit" disabled={busy} className="shrink-0">
          Invite
        </Button>
      </form>
      {invited && <Alert tone="success">{`Invite sent to ${invited}.`}</Alert>}
      {error && <Alert tone="danger">{error}</Alert>}
    </Card>
  )
}

/** Deactivates a person at the identity provider and here, after asking. */
function DeactivateButton({ user }: { user: AppUser }) {
  const deactivate = useDeactivateUser()
  const [confirming, setConfirming] = useState(false)
  const [error, setError] = useState('')

  async function run() {
    setError('')
    try {
      await deactivate(user.id)
      await mutate(swrKeys.userManagementUsers)
    } catch (err) {
      setError(failureText(err, 'Failed to deactivate.'))
    }
  }

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        aria-label={`Deactivate ${user.email}`}
        onClick={() => setConfirming(true)}
      >
        Deactivate
      </Button>
      <FieldError id={`deactivate-error-${user.id}`}>{error}</FieldError>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={`Deactivate ${user.email}?`}
        description="They are signed out everywhere and can no longer sign in."
        confirmLabel="Deactivate"
        destructive
        onConfirm={() => void run()}
      />
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

      <InviteForm />

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
                <DeactivateButton user={user} />
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
              <TableHead>
                <span className="sr-only">Actions</span>
              </TableHead>
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
                <TableCell colSpan={3 + APP_NAMES.length} className="py-6 text-center text-muted">
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
                  <TableCell>
                    <DeactivateButton user={user} />
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
