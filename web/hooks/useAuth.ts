import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { AuthService } from '@/lib/gen/auth/v1/auth_pb'
import { ProfileService } from '@/lib/gen/profile/v1/profile_pb'
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
  return (newPassword: string) => client.updatePassword({ newPassword })
}

export function useUpdateDisplayName() {
  const client = createServiceClient(ProfileService)
  return (displayName: string) => client.setDisplayName({ displayName })
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
  return () => client.mFAUnenroll({})
}

export function useCurrentUser() {
  const client = createServiceClient(AuthService)
  return useSWR(swrKeys.currentUser, () => client.getCurrentUser({}), {
    revalidateOnFocus: false,
    revalidateOnReconnect: false
  })
}
