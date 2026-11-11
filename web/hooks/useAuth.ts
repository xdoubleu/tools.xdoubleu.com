import useSWR from 'swr'
import { setTabPinsWrite } from '@/lib/account/offlineWrites'
import { createServiceClient } from '@/lib/client'
import { AuthService } from '@/lib/gen/auth/v1/auth_pb'
import { DashboardService } from '@/lib/gen/dashboard/v1/dashboard_pb'
import { enqueueWrite, sendWrite } from '@/lib/offline/outbox'
import { swrKeys } from '@/lib/swrKeys'

export function useSignIn() {
  const client = createServiceClient(AuthService)
  return (email: string, password: string, rememberMe: boolean, redirect: string) =>
    client.signIn({ email, password, rememberMe, redirect })
}

export function useSignOut() {
  const client = createServiceClient(AuthService)
  return () => client.signOut({})
}

export function useForgotPassword() {
  const client = createServiceClient(AuthService)
  return (email: string) => client.forgotPassword({ email })
}

export function useExchangeToken() {
  const client = createServiceClient(AuthService)
  return (accessToken: string, refreshToken: string) =>
    client.exchangeToken({ accessToken, refreshToken })
}

export function useUpdatePassword() {
  const client = createServiceClient(AuthService)
  return (currentPassword: string, newPassword: string) =>
    client.updatePassword({ currentPassword, newPassword })
}

export function useResetPassword() {
  const client = createServiceClient(AuthService)
  return (token: string, newPassword: string) => client.resetPassword({ token, newPassword })
}

export function useRegenerateRecoveryCodes() {
  const client = createServiceClient(AuthService)
  return (code: string) => client.regenerateRecoveryCodes({ code })
}

export function useUpdateDisplayName() {
  const client = createServiceClient(DashboardService)
  return (displayName: string) => client.setDisplayName({ displayName })
}

/** Saves the tab bar apps; the cached user shows them at once, even offline. Throws if the server refuses. */
export function useSetTabPins() {
  return async (tabPins: string[]) => {
    await sendWrite(enqueueWrite(setTabPinsWrite, { tabPins }))
  }
}

/** Reports the user switching to an app; fire and forget, since it only ranks default pins. */
export function useRecordAppOpen() {
  const client = createServiceClient(AuthService)
  return (app: string) => client.recordAppOpen({ app })
}

export function useMFAChallenge() {
  const client = createServiceClient(AuthService)
  return (code: string) => client.mFAChallenge({ code })
}

export function useMFAEnroll() {
  const client = createServiceClient(AuthService)
  return () => client.mFAEnroll({})
}

export function useMFAEnrollVerify() {
  const client = createServiceClient(AuthService)
  return (factorId: string, code: string) => client.mFAEnrollVerify({ factorId, code })
}

export function useMFAUnenroll() {
  const client = createServiceClient(AuthService)
  return (code: string) => client.mFAUnenroll({ code })
}

export function useCurrentUser() {
  const client = createServiceClient(AuthService)
  return useSWR(swrKeys.currentUser, () => client.getCurrentUser({}), {
    revalidateOnFocus: false
  })
}
