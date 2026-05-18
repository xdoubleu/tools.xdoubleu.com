import { createServiceClient } from '@/lib/client'
import { AuthService } from '@/lib/gen/auth/v1/auth_connect'

export function useSignIn() {
  const client = createServiceClient(AuthService)
  return (
    email: string,
    password: string,
    rememberMe: boolean,
    redirect: string
  ) => client.signIn({ email, password, rememberMe, redirect })
}

export function useSignOut() {
  const client = createServiceClient(AuthService)
  return () => client.signOut({})
}

export function useForgotPassword() {
  const client = createServiceClient(AuthService)
  return (email: string) => client.forgotPassword({ email })
}

export function useMFAEnroll() {
  const client = createServiceClient(AuthService)
  return () => client.mFAEnroll({})
}

export function useMFAEnrollVerify() {
  const client = createServiceClient(AuthService)
  return (factorId: string, code: string) =>
    client.mFAEnrollVerify({ factorId, code })
}

export function useMFAChallenge() {
  const client = createServiceClient(AuthService)
  return (code: string) => client.mFAChallenge({ code })
}
