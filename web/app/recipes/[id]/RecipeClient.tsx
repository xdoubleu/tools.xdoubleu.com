'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useRecipe, useDeleteRecipe, useShareRecipe, useUnshareRecipe } from '@/hooks/useRecipes'
import type { DeleteRecipeInput, ShareRecipeInput, UnshareRecipeInput } from '@/hooks/useRecipes'
import ShareModal from '@/components/recipes/ShareModal'

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
  const ingredients =
    scaledIngredients.length > 0
      ? scaledIngredients
      : (recipe?.ingredients ?? []).slice().sort((a, b) => a.sortOrder - b.sortOrder)

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

      {isLoading && <p>Loading recipe...</p>}
      {error && <p className="text-danger">Failed to load recipe.</p>}
      {recipe && (
        <>
          <div className="flex items-start justify-between mb-2">
            <h1 className="text-3xl font-bold">{recipe.name}</h1>
            {isOwner && (
              <div className="flex gap-2 ml-4 shrink-0">
                <button
                  onClick={() => setShowShareModal(true)}
                  className="px-3 py-1 border border-border rounded text-sm hover:bg-surface"
                >
                  Share
                </button>
                <Link
                  href={`/recipes/${recipe.id}/edit`}
                  className="px-3 py-1 border border-border rounded text-sm hover:bg-surface"
                >
                  Edit
                </Link>
                {deleteConfirm ? (
                  <div className="flex gap-2 items-center">
                    <button
                      onClick={handleDelete}
                      className="rounded-xl bg-danger px-3 py-1.5 text-sm text-white hover:opacity-90"
                    >
                      Confirm delete
                    </button>
                    <button
                      onClick={() => setDeleteConfirm(false)}
                      className="px-3 py-1 border border-border rounded-xl text-sm"
                    >
                      Cancel
                    </button>
                  </div>
                ) : (
                  <button
                    onClick={() => setDeleteConfirm(true)}
                    className="rounded-xl bg-danger px-3 py-1 text-sm text-white hover:opacity-90"
                  >
                    Delete
                  </button>
                )}
              </div>
            )}
          </div>

          <div className="flex items-center gap-3 mb-6 flex-wrap">
            <span className="text-muted text-sm">Serves</span>
            <input
              type="number"
              min="1"
              value={displayServings}
              onChange={(e) => handleServingsChange(parseInt(e.target.value, 10) || 1)}
              className="w-16 px-2 py-1 border border-input-border bg-input text-input-text rounded text-sm"
            />
            {servings > 0 && servings !== recipe.baseServings && (
              <button
                onClick={() => setServings(0)}
                className="text-xs text-accent hover:underline"
              >
                Reset to {recipe.baseServings}
              </button>
            )}
            {recipe.batchServings != null && (
              <span className="rounded-full bg-surface border border-border px-2.5 py-0.5 text-xs text-muted">
                Batch prep: {recipe.batchServings} servings
              </span>
            )}
          </div>

          {ingredients.length > 0 && (
            <section className="mb-6">
              <h2 className="text-xl font-semibold mb-3">Ingredients</h2>
              <ul>
                {ingredients.map((ing, idx) => (
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
