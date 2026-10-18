import { create } from '@bufbuild/protobuf'
import { IngredientUnitSchema } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { indexIngredientUnits, otherRecipeUnits } from '@/lib/recipes/unitConsistency'

const units = indexIngredientUnits(
  [
    { name: 'paprika', unit: 'stuk', recipeIds: ['a'] },
    { name: 'paprika', unit: 'Stuk', recipeIds: ['b'] },
    { name: 'paprika', unit: 'g', recipeIds: ['c'] },
    { name: 'ui', unit: '', recipeIds: ['a'] }
  ].map((u) => create(IngredientUnitSchema, u))
)

describe('otherRecipeUnits', () => {
  it('is undefined for an unknown ingredient', () => {
    expect(otherRecipeUnits('kaas', 'g', units, {})).toBeUndefined()
  })

  it('is undefined when another recipe uses the unit, ignoring case and spaces', () => {
    expect(otherRecipeUnits(' Paprika ', ' STUK', units, {})).toBeUndefined()
  })

  it('lists the other units when none matches', () => {
    expect(otherRecipeUnits('paprika', 'kg', units, {})).toEqual(['stuk', 'g'])
  })

  it('resolves the name through aliases', () => {
    expect(otherRecipeUnits('Rode paprika', 'kg', units, { 'rode paprika': 'paprika' })).toEqual([
      'stuk',
      'g'
    ])
  })

  it('ignores units only the edited recipe uses', () => {
    expect(otherRecipeUnits('paprika', 'kg', units, {}, 'c')).toEqual(['stuk'])
    expect(otherRecipeUnits('ui', 'stuk', units, {}, 'a')).toBeUndefined()
  })
})
