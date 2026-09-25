import useSWR from 'swr'
import { useCallback, useMemo } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import {
  TodoistService,
  type GetTodoistConnectionStatusResponse
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'

export function useTodoistConnection() {
  const client = createServiceClient(TodoistService)
  return useSWR<GetTodoistConnectionStatusResponse, Error>(swrKeys.todoistConnection, () =>
    client.getTodoistConnectionStatus({})
  )
}

// useConnectTodoist navigates to the authorize URL; the callback is a plain
// HTTP redirect route, so there's nothing to await.
export function useConnectTodoist() {
  const client = useMemo(() => createServiceClient(TodoistService), [])
  return useCallback(async () => {
    const resp = await client.connectTodoist({})
    window.location.href = resp.authorizeUrl
  }, [client])
}

export function useDisconnectTodoist() {
  const client = createServiceClient(TodoistService)
  return () => client.disconnectTodoist({})
}

export function useSendItemToTodoist() {
  const client = createServiceClient(TodoistService)
  return (itemId: string) => client.sendItemToTodoist({ itemId })
}
