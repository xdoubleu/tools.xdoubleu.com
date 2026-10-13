import { useSearchParams } from 'next/navigation'
import { isSharedList, LIST_PARAM } from '@/lib/listScope'

/** Whether the URL selects the family's shared list (?list=shared). */
export function useSharedList(): boolean {
  return isSharedList(useSearchParams()?.get(LIST_PARAM))
}
