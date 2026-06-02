'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { ConnectError } from '@connectrpc/connect'
import {
  useCurrentUser,
  useUpdatePassword,
  useMFAEnroll,
  useMFAEnrollVerify,
  useMFAUnenroll
} from '@/hooks/useAuth'

type MFAEnrollState = 'idle' | 'qr' | 'done'

export default function SettingsPage() {
  const { data, isLoading } = useCurrentUser()

  const updatePassword = useUpdatePassword()
  const mfaEnroll = useMFAEnroll()
  const mfaEnrollVerify = useMFAEnrollVerify()
  const mfaUnenroll = useMFAUnenroll()

  // Password section
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [pwSaving, setPwSaving] = useState(false)
  const [pwSaved, setPwSaved] = useState(false)
  const [pwError, setPwError] = useState('')

  // MFA section
  const [mfaState, setMfaState] = useState<MFAEnrollState>('idle')
  const [mfaQr, setMfaQr] = useState('')
  const [mfaSecret, setMfaSecret] = useState('')
  const [mfaFactorId, setMfaFactorId] = useState('')
  const [mfaCode, setMfaCode] = useState('')
  const [mfaBusy, setMfaBusy] = useState(false)
  const [mfaError, setMfaMfaError] = useState('')

  if (isLoading || !data) {
    return <p className="py-16 text-center text-sm text-muted">Loading…</p>
  }

  const hasMFA = data.hasMfa

  async function handlePasswordSave(e: React.FormEvent) {
    e.preventDefault()
    setPwSaved(false)
    setPwError('')
    if (newPassword !== confirmPassword) {
      setPwError('Passwords do not match.')
      return
    }
    if (newPassword.length < 8) {
      setPwError('Password must be at least 8 characters.')
      return
    }
    setPwSaving(true)
    try {
      await updatePassword(newPassword)
      setPwSaved(true)
      setNewPassword('')
      setConfirmPassword('')
    } catch (err) {
      if (err instanceof ConnectError) {
        setPwError(err.message)
      } else {
        setPwError('Failed to update password.')
      }
    } finally {
      setPwSaving(false)
    }
  }

  async function handleMFAEnable() {
    setMfaBusy(true)
    setMfaMfaError('')
    try {
      const res = await mfaEnroll()
      setMfaQr(res.qrSvg)
      setMfaSecret(res.secret)
      setMfaFactorId(res.factorId)
      setMfaState('qr')
    } catch (err) {
      if (err instanceof ConnectError) {
        setMfaMfaError(err.message)
      } else {
        setMfaMfaError('Failed to start MFA enrollment.')
      }
    } finally {
      setMfaBusy(false)
    }
  }

  async function handleMFAVerify(e: React.FormEvent) {
    e.preventDefault()
    setMfaBusy(true)
    setMfaMfaError('')
    try {
      await mfaEnrollVerify(mfaFactorId, mfaCode)
      await mutate('/auth/current-user')
      setMfaState('done')
    } catch (err) {
      if (err instanceof ConnectError) {
        setMfaMfaError(err.message)
      } else {
        setMfaMfaError('Invalid code. Please try again.')
      }
    } finally {
      setMfaBusy(false)
    }
  }

  async function handleMFADisable() {
    setMfaBusy(true)
    setMfaMfaError('')
    try {
      await mfaUnenroll()
      await mutate('/auth/current-user')
    } catch (err) {
      if (err instanceof ConnectError) {
        setMfaMfaError(err.message)
      } else {
        setMfaMfaError('Failed to disable MFA.')
      }
    } finally {
      setMfaBusy(false)
    }
  }

  return (
    <main className="mx-auto max-w-xl px-4 py-10 space-y-10">
      <h1 className="text-xl font-semibold text-fg">Account Settings</h1>

      {/* Password */}
      <section>
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-muted">
          Change Password
        </h2>

        {pwSaved && (
          <div className="mb-4 rounded-xl border border-success/30 bg-success/10 px-4 py-2 text-sm text-success">
            Password updated successfully.
          </div>
        )}
        {pwError && (
          <div className="mb-4 rounded-xl border border-danger/30 bg-danger/10 px-4 py-2 text-sm text-danger">
            {pwError}
          </div>
        )}

        <form onSubmit={handlePasswordSave} className="space-y-3">
          <div>
            <label htmlFor="new_password" className="mb-1 block text-sm text-subtle">
              New password
            </label>
            <input
              id="new_password"
              type="password"
              autoComplete="new-password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
              className="w-full rounded border border-border bg-surface px-3 py-2 text-sm text-fg placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-fg"
            />
          </div>
          <div>
            <label htmlFor="confirm_password" className="mb-1 block text-sm text-subtle">
              Confirm new password
            </label>
            <input
              id="confirm_password"
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              className="w-full rounded border border-border bg-surface px-3 py-2 text-sm text-fg placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-fg"
            />
          </div>
          <button
            type="submit"
            disabled={pwSaving}
            className="rounded bg-fg px-4 py-2 text-sm font-medium text-bg hover:opacity-80 disabled:opacity-50"
          >
            {pwSaving ? 'Updating…' : 'Update password'}
          </button>
        </form>
      </section>

      {/* MFA */}
      <section>
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-muted">
          Two-Factor Authentication
        </h2>

        {mfaError && (
          <div className="mb-4 rounded-xl border border-danger/30 bg-danger/10 px-4 py-2 text-sm text-danger">
            {mfaError}
          </div>
        )}

        {mfaState === 'done' && (
          <div className="mb-4 rounded-xl border border-success/30 bg-success/10 px-4 py-2 text-sm text-success">
            Two-factor authentication enabled successfully.
          </div>
        )}

        {hasMFA && mfaState === 'idle' ? (
          <div className="space-y-3">
            <p className="text-sm text-subtle">
              Two-factor authentication is <span className="font-medium text-fg">enabled</span>.
            </p>
            <button
              onClick={handleMFADisable}
              disabled={mfaBusy}
              className="rounded border border-danger/40 bg-danger/10 px-4 py-2 text-sm font-medium text-danger hover:bg-danger/20 disabled:opacity-50"
            >
              {mfaBusy ? 'Disabling…' : 'Disable MFA'}
            </button>
          </div>
        ) : mfaState === 'qr' ? (
          <div className="space-y-4">
            <p className="text-sm text-subtle">
              Scan this QR code with your authenticator app, then enter the 6-digit code below.
            </p>
            <div
              className="w-48 rounded border border-border bg-white p-2"
              dangerouslySetInnerHTML={{ __html: mfaQr }}
            />
            <p className="text-xs text-muted">
              Can&apos;t scan? Enter this key manually:{' '}
              <span className="font-mono text-fg">{mfaSecret}</span>
            </p>
            <form onSubmit={handleMFAVerify} className="space-y-3">
              <div>
                <label htmlFor="mfa_code" className="mb-1 block text-sm text-subtle">
                  Authenticator code
                </label>
                <input
                  id="mfa_code"
                  type="text"
                  inputMode="numeric"
                  maxLength={6}
                  value={mfaCode}
                  onChange={(e) => setMfaCode(e.target.value)}
                  required
                  className="w-full rounded border border-border bg-surface px-3 py-2 text-sm text-fg placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-fg"
                />
              </div>
              <div className="flex gap-2">
                <button
                  type="submit"
                  disabled={mfaBusy || mfaCode.length < 6}
                  className="rounded bg-fg px-4 py-2 text-sm font-medium text-bg hover:opacity-80 disabled:opacity-50"
                >
                  {mfaBusy ? 'Verifying…' : 'Verify & enable'}
                </button>
                <button
                  type="button"
                  onClick={() => setMfaState('idle')}
                  className="rounded px-4 py-2 text-sm text-muted hover:text-fg"
                >
                  Cancel
                </button>
              </div>
            </form>
          </div>
        ) : !hasMFA && mfaState !== 'done' ? (
          <div className="space-y-3">
            <p className="text-sm text-subtle">
              Two-factor authentication is <span className="font-medium text-fg">disabled</span>.
              Enable it for additional security.
            </p>
            <button
              onClick={handleMFAEnable}
              disabled={mfaBusy}
              className="rounded bg-fg px-4 py-2 text-sm font-medium text-bg hover:opacity-80 disabled:opacity-50"
            >
              {mfaBusy ? 'Loading…' : 'Enable MFA'}
            </button>
          </div>
        ) : null}
      </section>
    </main>
  )
}
