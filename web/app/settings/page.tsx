'use client'

import { useEffect, useState } from 'react'
import { mutate } from 'swr'
import { ConnectError } from '@connectrpc/connect'
import {
  useCurrentUser,
  useUpdatePassword,
  useUpdateDisplayName,
  useMFAEnroll,
  useMFAEnrollVerify,
  useMFAUnenroll,
  useRegenerateRecoveryCodes
} from '@/hooks/useAuth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert } from '@/components/ui/alert'
import { Field } from '@/components/ui/field'
import { PageHeader } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'
import { McpSetupSection } from '@/components/settings/McpSetupSection'
import { AppearanceSection } from '@/components/settings/AppearanceSection'
import RecoveryCodesDialog from '@/components/settings/RecoveryCodesDialog'

type MFAEnrollState = 'idle' | 'qr' | 'done'

export default function SettingsPage() {
  const { data, isLoading } = useCurrentUser()

  const updatePassword = useUpdatePassword()
  const updateDisplayName = useUpdateDisplayName()
  const mfaEnroll = useMFAEnroll()
  const mfaEnrollVerify = useMFAEnrollVerify()
  const mfaUnenroll = useMFAUnenroll()
  const regenerateRecoveryCodes = useRegenerateRecoveryCodes()

  const [displayName, setDisplayName] = useState('')
  const [nameSaving, setNameSaving] = useState(false)
  const [nameSaved, setNameSaved] = useState(false)
  const [nameError, setNameError] = useState('')

  useEffect(() => {
    if (data) setDisplayName(data.displayName)
  }, [data])

  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [pwSaving, setPwSaving] = useState(false)
  const [pwSaved, setPwSaved] = useState(false)
  const [pwError, setPwError] = useState('')

  const [mfaState, setMfaState] = useState<MFAEnrollState>('idle')
  const [mfaQr, setMfaQr] = useState('')
  const [mfaSecret, setMfaSecret] = useState('')
  const [mfaFactorId, setMfaFactorId] = useState('')
  const [mfaCode, setMfaCode] = useState('')
  // Step-up code for managing an enabled factor.
  const [factorCode, setFactorCode] = useState('')
  const [mfaBusy, setMfaBusy] = useState(false)
  const [mfaError, setMfaMfaError] = useState('')
  const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null)
  const [recoveryCodesError, setRecoveryCodesError] = useState('')

  if (isLoading || !data) {
    return <LoadingState className="py-16 text-center text-sm" />
  }

  const hasMFA = data.hasMfa

  async function handleDisplayNameSave(e: React.FormEvent) {
    e.preventDefault()
    setNameSaved(false)
    setNameError('')
    setNameSaving(true)
    try {
      await updateDisplayName(displayName.trim())
      await mutate(swrKeys.currentUser)
      setNameSaved(true)
    } catch (err) {
      if (err instanceof ConnectError) {
        setNameError(err.message)
      } else {
        setNameError('Failed to update display name.')
      }
    } finally {
      setNameSaving(false)
    }
  }

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
      await updatePassword(currentPassword, newPassword)
      setPwSaved(true)
      setCurrentPassword('')
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
      const res = await mfaEnrollVerify(mfaFactorId, mfaCode)
      await mutate(swrKeys.currentUser)
      setMfaState('done')
      if (res.recoveryCodes.length > 0) setRecoveryCodes(res.recoveryCodes)
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

  async function handleRegenerateRecoveryCodes() {
    setMfaBusy(true)
    setRecoveryCodesError('')
    try {
      const res = await regenerateRecoveryCodes(factorCode)
      setRecoveryCodes(res.recoveryCodes)
      setFactorCode('')
    } catch (err) {
      if (err instanceof ConnectError) {
        setRecoveryCodesError(err.message)
      } else {
        setRecoveryCodesError('Failed to regenerate recovery codes.')
      }
    } finally {
      setMfaBusy(false)
    }
  }

  async function handleMFADisable() {
    setMfaBusy(true)
    setMfaMfaError('')
    try {
      await mfaUnenroll(factorCode)
      setFactorCode('')
      await mutate(swrKeys.currentUser)
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
    <PageContainer size="narrow" className="space-y-10">
      <PageHeader title="Account Settings" className="mb-0" />

      <section>
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-muted">
          Display Name
        </h2>
        <p className="mb-4 text-sm text-subtle">
          Shown on your public profile when you share your books or games. Required before you can
          create a share link.
        </p>

        {nameSaved && (
          <Alert tone="success" className="mb-4">
            Display name updated successfully.
          </Alert>
        )}
        {nameError && (
          <Alert tone="danger" className="mb-4">
            {nameError}
          </Alert>
        )}

        <form onSubmit={handleDisplayNameSave} className="space-y-3">
          <Field label="Display name" htmlFor="display_name">
            <Input
              id="display_name"
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
            />
          </Field>
          <Button type="submit" size="sm" disabled={nameSaving || !displayName.trim()}>
            {nameSaving ? 'Saving…' : 'Save display name'}
          </Button>
        </form>
      </section>

      <section>
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-muted">
          Change Password
        </h2>

        {pwSaved && (
          <Alert tone="success" className="mb-4">
            Password updated successfully.
          </Alert>
        )}
        {pwError && (
          <Alert tone="danger" className="mb-4">
            {pwError}
          </Alert>
        )}

        <form onSubmit={handlePasswordSave} className="space-y-3">
          <Field label="Current password" htmlFor="current_password">
            <Input
              id="current_password"
              type="password"
              autoComplete="current-password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              required
            />
          </Field>
          <Field label="New password" htmlFor="new_password">
            <Input
              id="new_password"
              type="password"
              autoComplete="new-password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
            />
          </Field>
          <Field label="Confirm new password" htmlFor="confirm_password">
            <Input
              id="confirm_password"
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
            />
          </Field>
          <Button type="submit" size="sm" disabled={pwSaving}>
            {pwSaving ? 'Updating…' : 'Update password'}
          </Button>
        </form>
      </section>

      <section>
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-muted">
          Two-Factor Authentication
        </h2>

        {mfaError && (
          <Alert tone="danger" className="mb-4">
            {mfaError}
          </Alert>
        )}

        {mfaState === 'done' && (
          <Alert tone="success" className="mb-4">
            Two-factor authentication enabled successfully.
          </Alert>
        )}

        {hasMFA && mfaState === 'idle' ? (
          <div className="space-y-3">
            <p className="text-sm text-subtle">
              Two-factor authentication is <span className="font-medium text-fg">enabled</span>.
            </p>
            {recoveryCodesError && <Alert tone="danger">{recoveryCodesError} </Alert>}
            <Field label="Authenticator or recovery code" htmlFor="factor_code">
              <Input
                id="factor_code"
                type="text"
                autoComplete="one-time-code"
                value={factorCode}
                onChange={(e) => setFactorCode(e.target.value.trim())}
              />
            </Field>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={handleRegenerateRecoveryCodes}
                disabled={mfaBusy || !factorCode}
              >
                {mfaBusy ? 'Generating…' : 'Regenerate recovery codes'}
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={handleMFADisable}
                disabled={mfaBusy || !factorCode}
              >
                {mfaBusy ? 'Disabling…' : 'Disable MFA'}
              </Button>
            </div>
          </div>
        ) : mfaState === 'qr' ? (
          <div className="space-y-4">
            <p className="text-sm text-subtle">
              Scan this QR code with your authenticator app, then enter the 6-digit code below.
            </p>
            <div
              className="w-48 rounded-xl border border-border bg-white p-2"
              dangerouslySetInnerHTML={{ __html: mfaQr }}
            />
            <p className="text-xs text-muted">
              Can&apos;t scan? Enter this key manually:{' '}
              <span className="font-mono text-fg">{mfaSecret}</span>
            </p>
            <form onSubmit={handleMFAVerify} className="space-y-3">
              <Field label="Authenticator code" htmlFor="mfa_code">
                <Input
                  id="mfa_code"
                  type="text"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  maxLength={6}
                  value={mfaCode}
                  onChange={(e) => setMfaCode(e.target.value)}
                  required
                />
              </Field>
              <div className="flex flex-wrap gap-2">
                <Button type="submit" size="sm" disabled={mfaBusy || mfaCode.length < 6}>
                  {mfaBusy ? 'Verifying…' : 'Verify & enable'}
                </Button>
                <Button type="button" variant="ghost" size="sm" onClick={() => setMfaState('idle')}>
                  Cancel
                </Button>
              </div>
            </form>
          </div>
        ) : !hasMFA && mfaState !== 'done' ? (
          <div className="space-y-3">
            <p className="text-sm text-subtle">
              Two-factor authentication is <span className="font-medium text-fg">disabled</span>.
              Enable it for additional security.
            </p>
            <Button size="sm" onClick={handleMFAEnable} disabled={mfaBusy}>
              {mfaBusy ? 'Loading…' : 'Enable MFA'}
            </Button>
          </div>
        ) : null}
      </section>

      <AppearanceSection />

      <section>
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-muted">Privacy</h2>
        <p className="text-sm text-subtle">
          Usage analytics and session recording are enabled to help improve the app.
        </p>
      </section>

      <McpSetupSection />

      {recoveryCodes && (
        <RecoveryCodesDialog codes={recoveryCodes} onDismiss={() => setRecoveryCodes(null)} />
      )}
    </PageContainer>
  )
}
