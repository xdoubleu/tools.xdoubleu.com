import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { FamilyService } from '@/lib/gen/family/v1/family_pb'
import type { GetFamilyResponse } from '@/lib/gen/family/v1/family_pb'
import { swrKeys } from '@/lib/swrKeys'

export function useFamily() {
  const client = createServiceClient(FamilyService)
  return useSWR<GetFamilyResponse, Error>(swrKeys.family, () => client.getFamily({}))
}

export function useInviteToFamily() {
  const client = createServiceClient(FamilyService)
  return (email: string) => client.inviteToFamily({ email })
}

export function useAcceptFamilyInvite() {
  const client = createServiceClient(FamilyService)
  return () => client.acceptFamilyInvite({})
}

export function useDeclineFamilyInvite() {
  const client = createServiceClient(FamilyService)
  return () => client.declineFamilyInvite({})
}

export function useSetFamilyDisplayName() {
  const client = createServiceClient(FamilyService)
  return (displayName: string) => client.setFamilyDisplayName({ displayName })
}

export function useLeaveFamily() {
  const client = createServiceClient(FamilyService)
  return () => client.leaveFamily({})
}
