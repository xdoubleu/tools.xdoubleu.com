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

// useConnectTodoist returns a function that fetches an authorize URL and
// navigates the browser there — the OAuth callback that completes the flow
// is a plain HTTP redirect route (see api/apps/learningpaths/routes.go), not
// a ConnectRPC method, so there's nothing more for the client to await.
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
