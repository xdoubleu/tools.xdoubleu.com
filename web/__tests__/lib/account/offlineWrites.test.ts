import { create } from '@bufbuild/protobuf'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'
import { accountWrites, setTabPinsWrite } from '@/lib/account/offlineWrites'
import { swrKeys } from '@/lib/swrKeys'

const apply = (key: unknown, data: unknown, tabPins: string[]) =>
  setTabPinsWrite.apply(key, data, setTabPinsWrite.encode({ tabPins }), undefined)

describe('setTabPinsWrite', () => {
  it('puts the pins on the cached current user and marks them chosen', () => {
    const user = create(GetCurrentUserResponseSchema, { role: 'user' })
    const result = apply(swrKeys.currentUser, user, ['feeds', 'books'])
    expect(result).toMatchObject({
      role: 'user',
      tabPins: ['feeds', 'books'],
      tabPinsCustomised: true
    })
  })

  it('keeps a deliberate empty choice', () => {
    const user = create(GetCurrentUserResponseSchema, { tabPins: ['books'] })
    expect(apply(swrKeys.currentUser, user, [])).toMatchObject({
      tabPins: [],
      tabPinsCustomised: true
    })
  })

  it('leaves other keys and other data untouched', () => {
    const user = create(GetCurrentUserResponseSchema, {})
    expect(apply('/feeds', user, ['books'])).toBe(user)
    expect(apply(swrKeys.currentUser, 'nope', ['books'])).toBe('nope')
    expect(apply(swrKeys.currentUser, undefined, ['books'])).toBeUndefined()
  })

  it('names the change and collapses repeated edits', () => {
    const req = setTabPinsWrite.encode({ tabPins: ['books'] })
    expect(setTabPinsWrite.describe(req)).toBe('Change the tab bar apps')
    expect(setTabPinsWrite.coalesceKey(req)).toBe('tab-pins')
    expect(setTabPinsWrite.revalidate).toBe(swrKeys.currentUser)
    expect(setTabPinsWrite.revalidateOnSuccess).toBe(false)
    expect(accountWrites).toEqual([setTabPinsWrite])
  })
})
