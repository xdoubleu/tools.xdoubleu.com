import { createServiceClient } from '@/lib/client'
import { AuthService } from '@/lib/gen/auth/v1/auth_pb'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_pb'

describe('createServiceClient', () => {
  it('returns the same client instance for the same service', () => {
    const a = createServiceClient(AuthService)
    const b = createServiceClient(AuthService)
    expect(a).toBe(b)
  })

  it('returns distinct clients for distinct services', () => {
    const a = createServiceClient(AuthService)
    const b = createServiceClient(RecipesService)
    expect(a).not.toBe(b)
  })

  it('exposes the service methods', () => {
    const client = createServiceClient(AuthService)
    expect(typeof client.signIn).toBe('function')
  })
})
