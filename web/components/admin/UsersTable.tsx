'use client'

import { useSetRole, useSetAppAccess } from '@/hooks/useAdmin'
import type { AppUser } from '@/lib/gen/admin/v1/admin_pb'

interface UsersTableProps {
  users: AppUser[]
  onUpdated?: () => void
}

const APPS = ['backlog', 'todos', 'recipes', 'contacts', 'watchparty', 'icsproxy']

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
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b">
            <th className="text-left p-3">Email</th>
            <th className="text-left p-3">Role</th>
            {APPS.map((app) => (
              <th key={app} className="text-center p-3 text-xs">
                {app}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {users.map((user) => (
            <tr key={user.id} className="border-b hover:bg-surface">
              <td className="p-3">{user.email}</td>
              <td className="p-3">
                <select
                  value={user.role || 'user'}
                  onChange={(e) => handleRoleChange(user.id, e.target.value)}
                  className="px-2 py-1 rounded border border-input-border bg-input text-input-text text-sm"
                >
                  <option value="user">User</option>
                  <option value="admin">Admin</option>
                </select>
              </td>
              {APPS.map((app) => (
                <td key={app} className="p-3 text-center">
                  <input
                    type="checkbox"
                    checked={(user.appAccess || []).includes(app)}
                    onChange={(e) => handleAppAccessChange(user.id, app, e.target.checked)}
                    className="w-4 h-4"
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
