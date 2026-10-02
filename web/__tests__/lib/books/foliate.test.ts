import { createFoliateView } from '@/lib/books/foliate'

describe('createFoliateView', () => {
  it('loads foliate-js from its static copy and creates its view element', async () => {
    const load = jest.fn().mockResolvedValue({})
    const view = await createFoliateView(load)
    expect(load).toHaveBeenCalledWith('/foliate-js/view.js')
    expect(view.tagName).toBe('FOLIATE-VIEW')
  })

  it('imports the static copy by default, which only the browser can reach', async () => {
    await expect(createFoliateView()).rejects.toThrow(/foliate-js\/view/)
  })
})
