import type { IngredientUnit } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

const normalize = (value: string) => value.trim().toLowerCase()

/** Merged ingredient name → unit → recipe IDs using it. */
export type UnitsByName = Map<string, Map<string, string[]>>

export function indexIngredientUnits(units: IngredientUnit[]): UnitsByName {
  const byName: UnitsByName = new Map()
  for (const u of units) {
    const byUnit = byName.get(u.name) ?? new Map<string, string[]>()
    const key = normalize(u.unit)
    byUnit.set(key, [...(byUnit.get(key) ?? []), ...u.recipeIds])
    byName.set(u.name, byUnit)
  }
  return byName
}

/**
 * The units other recipes use for this (merged) ingredient when none matches
 * `unit`, else undefined. `recipeId` is the recipe being edited, if saved.
 */
export function otherRecipeUnits(
  name: string,
  unit: string,
  unitsByName: UnitsByName,
  aliases: Record<string, string>,
  recipeId?: string
): string[] | undefined {
  const key = normalize(name)
  const byUnit = unitsByName.get(aliases[key] ?? key)
  if (!byUnit) return undefined

  const others = [...byUnit.entries()]
    .filter(([, recipeIds]) => recipeIds.some((id) => id !== recipeId))
    .map(([u]) => u)
  if (others.length === 0 || others.includes(normalize(unit))) return undefined
  return others
}
