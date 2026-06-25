'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useRecipe, useDeleteRecipe } from '@/hooks/useRecipes'
import type { DeleteRecipeInput } from '@/hooks/useRecipes'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default function RecipeClient({ id }: { id: string }) {
  const [servings, setServings] = useState(0)
  const { data, error, isLoading } = useRecipe(id, servings || undefined)
  const deleteRecipe = useDeleteRecipe()
  const router = useRouter()

  const [deleteConfirm, setDeleteConfirm] = useState(false)

  const recipe = data?.recipe
  const isOwner = data?.isOwner ?? false
  const canEdit = data?.canEdit ?? false
  const displayServings = servings || recipe?.baseServings || 1
  const scaledIngredients = data?.scaledIngredients ?? []
  const sortedIngredients = (recipe?.ingredients ?? [])
    .slice()
    .sort((a, b) => a.sortOrder - b.sortOrder)

  type DisplayIngredient = {
    id: string
    name: string
    amount: string | number
    unit: string
    groupName?: string
  }
  const displayIngredients: DisplayIngredient[] =
    scaledIngredients.length > 0
      ? scaledIngredients.map((scaled, idx) => ({
          id: sortedIngredients[idx]?.id ?? String(idx),
          name: scaled.name,
          amount: scaled.amount,
          unit: scaled.unit,
          groupName: sortedIngredients[idx]?.groupName
        }))
      : sortedIngredients.map((ing) => ({
          id: ing.id,
          name: ing.name,
          amount: ing.amount,
          unit: ing.unit,
          groupName: ing.groupName
        }))

  type GroupedSection = { groupName: string | undefined; items: DisplayIngredient[] }
  const groupedIngredients = displayIngredients.reduce<GroupedSection[]>((acc, ing) => {
    const group = ing.groupName ?? undefined
    const existing = acc.find((s) => s.groupName === group)
    if (existing) {
      existing.items.push(ing)
    } else {
      acc.push({ groupName: group, items: [ing] })
    }
    return acc
  }, [])

  const handleServingsChange = (val: number) => {
    setServings(val <= 0 || val === recipe?.baseServings ? 0 : val)
  }

  const handleDelete = async () => {
    if (!recipe) return
    const req: DeleteRecipeInput = { id: recipe.id }
    await deleteRecipe(req)
    router.push('/recipes/list')
  }

  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Recipes', href: '/recipes/list' }, { label: recipe?.name ?? 'Recipe' }]}
      />

      {isLoading && !recipe && <p>Loading recipe...</p>}
      {error && <p className="text-danger">Failed to load recipe.</p>}
      {recipe && (
        <>
          <div className="flex items-start justify-between mb-2">
            <h1 className="text-3xl font-bold">{recipe.name}</h1>
            {canEdit && (
              <div className="flex gap-2 ml-4 shrink-0">
                <Button asChild variant="secondary" size="sm">
                  <Link href={`/recipes/${recipe.id}/edit`}>Edit</Link>
                </Button>
                {isOwner &&
                  (deleteConfirm ? (
                    <div className="flex gap-2 items-center">
                      <Button variant="destructive" size="sm" onClick={handleDelete}>
                        Confirm delete
                      </Button>
                      <Button variant="secondary" size="sm" onClick={() => setDeleteConfirm(false)}>
                        Cancel
                      </Button>
                    </div>
                  ) : (
                    <Button variant="destructive" size="sm" onClick={() => setDeleteConfirm(true)}>
                      Delete
                    </Button>
                  ))}
              </div>
            )}
          </div>

          <div className="flex items-center gap-3 mb-6 flex-wrap">
            <span className="text-muted text-sm">Serves</span>
            <Input
              type="number"
              min="1"
              value={displayServings}
              onChange={(e) => handleServingsChange(parseInt(e.target.value, 10) || 1)}
              className="h-9 w-16 px-2"
            />
            {servings > 0 && servings !== recipe.baseServings && (
              <Button
                variant="link"
                size="sm"
                onClick={() => setServings(0)}
                className="h-auto px-0 text-xs"
              >
                Reset to {recipe.baseServings}
              </Button>
            )}
            {recipe.batchServings != null && (
              <span className="rounded-full bg-surface border border-border px-2.5 py-0.5 text-xs text-muted">
                Batch prep: {recipe.batchServings} servings
              </span>
            )}
          </div>

          {displayIngredients.length > 0 && (
            <section className="mb-6">
              <h2 className="text-xl font-semibold mb-3">Ingredients</h2>
              <div className="space-y-3">
                {groupedIngredients.map((section, sIdx) => {
                  const list = (
                    <ul>
                      {section.items.map((ing, idx) => (
                        <li
                          key={'id' in ing ? ing.id : idx}
                          className="flex gap-2 py-1 border-b last:border-0 border-border"
                        >
                          <span className="font-medium">
                            {ing.amount} {ing.unit}
                          </span>
                          <span>{ing.name}</span>
                        </li>
                      ))}
                    </ul>
                  )
                  return (
                    <div
                      key={sIdx}
                      className="rounded-2xl border border-border bg-surface/50 overflow-hidden"
                    >
                      {section.groupName && (
                        <h3 className="bg-surface px-3 py-1.5 text-sm font-semibold text-subtle border-b border-border">
                          {section.groupName}
                        </h3>
                      )}
                      <div className="px-3 py-1">{list}</div>
                    </div>
                  )
                })}
              </div>
            </section>
          )}

          {recipe.instructions && (
            <section className="mb-6">
              <h2 className="text-xl font-semibold mb-3">Instructions</h2>
              <div className="prose max-w-none whitespace-pre-wrap text-subtle">
                {recipe.instructions}
              </div>
            </section>
          )}
        </>
      )}
    </PageContainer>
  )
}
