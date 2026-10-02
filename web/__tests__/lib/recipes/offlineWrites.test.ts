import { create, type DescMessage, type MessageInitShape } from '@bufbuild/protobuf'
import type { OfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import { GetRecipeResponseSchema, ListRecipesResponseSchema } from '@/lib/gen/recipes/v1/recipes_pb'
import {
  createRecipeWrite,
  deleteRecipeWrite,
  recipeWrites,
  updateRecipeWrite
} from '@/lib/recipes/offlineWrites'

function run<I extends DescMessage>(
  write: OfflineWrite<I>,
  init: MessageInitShape<I>,
  key: unknown,
  data: unknown
) {
  return write.apply(key, data, write.encode(init), undefined)
}

const list = () =>
  create(ListRecipesResponseSchema, {
    recipes: [
      { id: 'b', name: 'Bread', baseServings: 2, userId: 'u1' },
      { id: 's', name: 'Soup', baseServings: 4 }
    ],
    hasMore: true
  })

const fields = {
  name: 'Apple pie',
  steps: [' Peel ', '', 'Bake'],
  baseServings: 0,
  ingredientNames: ['apples', '', 'sugar'],
  ingredientAmounts: [3, 1, 0.5],
  ingredientUnits: ['', 'x', ' cup '],
  ingredientGroupNames: ['', '', ' Filling ']
}

describe('recipe offline writes', () => {
  it('leave unrelated keys and data untouched', () => {
    const data = list()
    for (const write of recipeWrites) {
      expect(write.apply('/feeds', data, new Uint8Array(), undefined)).toBe(data)
    }
    expect(run(deleteRecipeWrite, { id: 'b' }, swrKeys.recipes, 'nope')).toBe('nope')
  })

  it('adds a created recipe to the list the way the API stores it', () => {
    const result = run(createRecipeWrite, { id: 'a', ...fields }, swrKeys.recipes, list())

    expect(result).toMatchObject({
      hasMore: true,
      recipes: [
        {
          id: 'a',
          name: 'Apple pie',
          instructions: 'Peel\nBake',
          baseServings: 2,
          ingredients: [
            { name: 'apples', amount: 3, unit: '', sortOrder: 0 },
            { name: 'sugar', amount: 0.5, unit: 'cup', sortOrder: 2, groupName: 'Filling' }
          ]
        },
        { id: 'b' },
        { id: 's' }
      ]
    })
  })

  it('updates a listed recipe, keeping its owner, and ignores unlisted ones', () => {
    expect(
      run(
        updateRecipeWrite,
        { id: 'b', ...fields, name: 'Zucchini bread' },
        swrKeys.recipes,
        list()
      )
    ).toMatchObject({
      recipes: [{ id: 's' }, { id: 'b', name: 'Zucchini bread', userId: 'u1' }]
    })

    const data = list()
    expect(run(updateRecipeWrite, { id: 'x', ...fields }, swrKeys.recipes, data)).toBe(data)
  })

  it('updates a cached detail response and rescales its ingredients', () => {
    const detail = create(GetRecipeResponseSchema, {
      recipe: { id: 'b', name: 'Bread', baseServings: 2, userId: 'u1' },
      servings: 6,
      isOwner: true,
      canEdit: true
    })

    const result = run(
      updateRecipeWrite,
      { id: 'b', ...fields, baseServings: 2 },
      swrKeys.recipe('b', 6),
      detail
    )

    expect(result).toMatchObject({
      servings: 6,
      canEdit: true,
      recipe: { name: 'Apple pie', userId: 'u1' },
      scaledIngredients: [
        { name: 'apples', amount: '9', unit: '' },
        { name: 'sugar', amount: '1½', unit: 'cup' }
      ]
    })
  })

  it('scales to the base servings when the detail has none', () => {
    const detail = create(GetRecipeResponseSchema, { recipe: { id: 'b' }, servings: 0 })

    expect(
      run(updateRecipeWrite, { id: 'b', ...fields, baseServings: 4 }, swrKeys.recipe('b'), detail)
    ).toMatchObject({ servings: 4, scaledIngredients: [{ amount: '3' }, { amount: '½' }] })
    expect(run(updateRecipeWrite, { id: 'b', ...fields }, swrKeys.recipe('other'), detail)).toBe(
      detail
    )
  })

  it('removes a deleted recipe from the list', () => {
    expect(run(deleteRecipeWrite, { id: 'b' }, swrKeys.recipes, list())).toMatchObject({
      recipes: [{ id: 's' }]
    })
  })

  it('describes each write', () => {
    expect(createRecipeWrite.describe(createRecipeWrite.encode({ name: 'Pie' }))).toBe(
      'Add recipe “Pie”'
    )
    expect(updateRecipeWrite.describe(updateRecipeWrite.encode({ name: 'Pie' }))).toBe(
      'Edit recipe “Pie”'
    )
    expect(deleteRecipeWrite.describe(deleteRecipeWrite.encode({}))).toBe('Delete a recipe')
  })

  it('names ingredient IDs after the recipe and tolerates short arrays', () => {
    const result = run(
      createRecipeWrite,
      { id: 'a', name: 'Toast', ingredientNames: ['bread'], ingredientAmounts: [] },
      swrKeys.recipes,
      list()
    )
    expect(result).toMatchObject({
      recipes: [
        { id: 'b' },
        { id: 's' },
        { id: 'a', ingredients: [{ id: 'a-0', recipeId: 'a', amount: 0, unit: '' }] }
      ]
    })
  })

  it('ignores tuple keys and list data under other keys', () => {
    const detail = create(GetRecipeResponseSchema, { recipe: { id: 'b' } })
    expect(run(updateRecipeWrite, { id: 'b', ...fields }, ['/recipes/b'], detail)).toBe(detail)

    const data = list()
    expect(run(updateRecipeWrite, { id: 'b', ...fields }, '/feeds', data)).toBe(data)
  })

  it('refetches recipes and registers every write', () => {
    expect(recipeWrites).toEqual([createRecipeWrite, updateRecipeWrite, deleteRecipeWrite])
    for (const write of recipeWrites) expect(write.revalidate).toBe('/recipes')
  })
})
