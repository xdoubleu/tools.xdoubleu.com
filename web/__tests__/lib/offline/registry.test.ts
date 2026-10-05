import { create } from '@bufbuild/protobuf'
import { defineOfflineWrite } from '@/lib/offline/registry'
import { keepaliveTransport, transport } from '@/lib/client'
import {
  DeleteShoppingItemRequestSchema,
  ShoppingListService
} from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

jest.mock('@/lib/client', () => ({
  transport: { unary: jest.fn(async () => ({})) },
  keepaliveTransport: { unary: jest.fn(async () => ({})) }
}))

describe('defineOfflineWrite', () => {
  const apply = jest.fn((_key: unknown, data: unknown) => data)
  const write = defineOfflineWrite({
    method: ShoppingListService.method.deleteShoppingItem,
    apply,
    describe: (req) => `Delete ${req.itemId}`,
    revalidate: '/shoppinglist'
  })
  const bytes = write.encode({ itemId: 'i1' })

  beforeEach(() => jest.clearAllMocks())

  it('identifies the write by service and method', () => {
    expect(write.id).toBe('shoppinglist.v1.ShoppingListService/DeleteShoppingItem')
    expect(write.revalidate).toBe('/shoppinglist')
  })

  it('decodes the stored request for apply and describe', () => {
    write.apply('/k', 'data', bytes, 'hint')

    expect(apply).toHaveBeenCalledWith(
      '/k',
      'data',
      create(DeleteShoppingItemRequestSchema, { itemId: 'i1' }),
      'hint'
    )
    expect(write.describe(bytes)).toBe('Delete i1')
  })

  it('sends the decoded request over the shared transport', async () => {
    await write.send(bytes)

    expect(transport.unary).toHaveBeenCalledWith(
      ShoppingListService.method.deleteShoppingItem,
      undefined,
      undefined,
      undefined,
      create(DeleteShoppingItemRequestSchema, { itemId: 'i1' })
    )
  })

  it('has no coalesce key unless the spec defines one', () => {
    expect(write.coalesceKey(bytes)).toBeUndefined()
  })

  it('coalesces by the spec key and sends keepalive writes on the keepalive transport', async () => {
    const latest = defineOfflineWrite({
      method: ShoppingListService.method.deleteShoppingItem,
      apply,
      describe: () => 'x',
      revalidate: '/shoppinglist',
      coalesce: (req) => req.itemId,
      keepalive: true
    })

    expect(latest.coalesceKey(bytes)).toBe('i1')
    await latest.send(bytes)
    expect(keepaliveTransport.unary).toHaveBeenCalledTimes(1)
    expect(transport.unary).not.toHaveBeenCalled()
  })
})
