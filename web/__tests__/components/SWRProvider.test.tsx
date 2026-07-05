import { render, screen } from '@testing-library/react'
import useSWR from 'swr'
import { create } from '@bufbuild/protobuf'
import SWRProvider from '@/components/SWRProvider'
import { swrKeys } from '@/lib/swrKeys'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'
import type { GetCurrentUserResponse } from '@/lib/gen/auth/v1/auth_pb'

function Probe() {
  const { data } = useSWR<GetCurrentUserResponse>(
    swrKeys.currentUser,
    () => new Promise<never>(() => {})
  )
  return <span>{data ? `role:${data.role}` : 'no-user'}</span>
}

describe('SWRProvider', () => {
  it('exposes the server-fetched user as fallback for the current-user key', () => {
    const user = create(GetCurrentUserResponseSchema, {
      role: 'admin',
      appAccess: [],
      hasMfa: false
    })

    render(
      <SWRProvider currentUser={user}>
        <Probe />
      </SWRProvider>
    )

    expect(screen.getByText('role:admin')).toBeInTheDocument()
  })

  it('provides no fallback when the server fetch returned null', () => {
    render(
      <SWRProvider currentUser={null}>
        <Probe />
      </SWRProvider>
    )

    expect(screen.getByText('no-user')).toBeInTheDocument()
  })
})
