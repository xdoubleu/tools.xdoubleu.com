'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useRecipe, useDeleteRecipe, useShareRecipe, useUnshareRecipe } from '@/hooks/useRecipes'
import type { DeleteRecipeInput, ShareRecipeInput, UnshareRecipeInput } from '@/hooks/useRecipes'
import ShareModal from '@/components/recipes/ShareModal'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export default function RecipeClient({ id }: { id: string }) {
  const [servings, setServings] = useState(0)
  const { data, error, isLoading, mutate } = useRecipe(id, servings || undefined)
  const deleteRecipe = useDeleteRecipe()
  const shareRecipe = useShareRecipe()
  const unshareRecipe = useUnshareRecipe()
  const router = useRouter()

  const [showShareModal, setShowShareModal] = useState(false)
  const [deleteConfirm, setDeleteConfirm] = useState(false)

  const recipe = data?.recipe
  const isOwner = data?.isOwner ?? false
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
    const last = acc[acc.length - 1]
    if (last && last.groupName === group) {
      last.items.push(ing)
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

  const handleShare = async (userId: string) => {
    if (!recipe) return
    const req: ShareRecipeInput = { id: recipe.id, contactUserId: userId }
    await shareRecipe(req)
    await mutate()
  }

  const handleUnshare = async (userId: string) => {
    if (!recipe) return
    const req: UnshareRecipeInput = { id: recipe.id, targetUserId: userId }
    await unshareRecipe(req)
    await mutate()
  }

  return (
    <main className="max-w-2xl mx-auto p-6">
      <Link href="/recipes/list" className="mb-4 block text-sm text-accent hover:underline">
        &larr; Back to recipes
      </Link>

      {isLoading && !recipe && <p>Loading recipe...</p>}
      {error && <p className="text-danger">Failed to load recipe.</p>}
      {recipe && (
        <>
          <div className="flex items-start justify-between mb-2">
            <h1 className="text-3xl font-bold">{recipe.name}</h1>
            {isOwner && (
              <div className="flex gap-2 ml-4 shrink-0">
                <Button variant="secondary" size="sm" onClick={() => setShowShareModal(true)}>
                  Share
                </Button>
                <Button asChild variant="secondary" size="sm">
                  <Link href={`/recipes/${recipe.id}/edit`}>Edit</Link>
                </Button>
                {deleteConfirm ? (
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
                )}
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
              {groupedIngredients.map((section, sIdx) => (
                <div key={sIdx}>
                  {section.groupName && (
                    <p className="text-sm font-semibold text-muted mt-3 mb-1">
                      {section.groupName}
                    </p>
                  )}
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
                </div>
              ))}
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

          {showShareModal && (
            <ShareModal
              sharedWith={recipe.sharedWith}
              onShare={handleShare}
              onUnshare={handleUnshare}
              onClose={() => setShowShareModal(false)}
            />
          )}
        </>
      )}
    </main>
  )
}
