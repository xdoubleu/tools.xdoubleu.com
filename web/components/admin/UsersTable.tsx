'use client'

import { useSetRole, useSetAppAccess } from '@/hooks/useAdmin'
import type { AppUser } from '@/lib/gen/admin/v1/admin_pb'
import { Select } from '@/components/ui/select'

interface UsersTableProps {
  users: AppUser[]
  onUpdated?: () => void
}

const APPS = [
  'games',
  'books',
  'todos',
  'recipes',
  'mealplans',
  'shoppinglist',
  'contacts',
  'watchparty',
  'icsproxy'
]

export default function UsersTable({ users, onUpdated }: UsersTableProps) {
  const setRole = useSetRole()
  const setAppAccess = useSetAppAccess()

  const handleRoleChange = async (userId: string, role: string) => {
    try {
      await setRole(userId, role)
      onUpdated?.()
    } catch (err) {
      console.error('Failed to set role:', err)
    }
  }

  const handleAppAccessChange = async (userId: string, appName: string, grant: boolean) => {
    try {
      await setAppAccess(userId, appName, grant)
      onUpdated?.()
    } catch (err) {
      console.error('Failed to set app access:', err)
    }
  }

  return (
    <div className="overflow-x-auto rounded-2xl border border-border shadow-card">
      <table className="w-full text-sm">
        <thead className="bg-surface">
          <tr className="border-b border-border">
            <th className="p-3 text-left font-medium text-subtle">Email</th>
            <th className="p-3 text-left font-medium text-subtle">Role</th>
            {APPS.map((app) => (
              <th key={app} className="p-3 text-center text-xs font-medium text-subtle">
                {app}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-card">
          {users.map((user) => (
            <tr key={user.id} className="border-b border-border hover:bg-hover transition-colors">
              <td className="p-3 text-fg">{user.email}</td>
              <td className="p-3">
                <Select
                  value={user.role || 'user'}
                  onChange={(e) => handleRoleChange(user.id, e.target.value)}
                >
                  <option value="user">User</option>
                  <option value="admin">Admin</option>
                </Select>
              </td>
              {APPS.map((app) => (
                <td key={app} className="p-3 text-center">
                  <input
                    type="checkbox"
                    checked={(user.appAccess || []).includes(app)}
                    onChange={(e) => handleAppAccessChange(user.id, app, e.target.checked)}
                    className="h-4 w-4 accent-[rgb(var(--color-accent))]"
                  />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
