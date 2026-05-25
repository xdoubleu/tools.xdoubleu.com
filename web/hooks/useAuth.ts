import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { AuthService } from '@/lib/gen/auth/v1/auth_connect'

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

export function useMFAChallenge() {
  const client = createServiceClient(AuthService)
  return (code: string) => client.mFAChallenge({ code })
}

export function useCurrentUser() {
  const client = createServiceClient(AuthService)
  return useSWR('/auth/current-user', () => client.getCurrentUser({}), {
    revalidateOnFocus: false,
    revalidateOnReconnect: false
  })
}
