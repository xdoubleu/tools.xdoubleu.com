'use client'

import { useMemo } from 'react'
import Link from 'next/link'
import { useRecipes, useFetchRecipesPage } from '@/hooks/useRecipes'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { cn } from '@/lib/cn'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function RecipeCard({ recipe }: { recipe: Recipe }) {
  return (
    <Link href={`/recipes/${recipe.id}`} className={cn(interactiveCardClass, 'relative block p-4')}>
      <CardLinkStatus />
      <h2 className="font-semibold text-lg">{recipe.name}</h2>
      <p className="text-sm text-muted mt-1">
        Serves {recipe.baseServings}
        {recipe.isDraft && (
          <Badge variant="secondary" className="ml-2">
            Draft
          </Badge>
        )}
      </p>
    </Link>
  )
}

export default function RecipesListClient() {
  const { data, error, isLoading } = useRecipes()

  const fetchPage = useFetchRecipesPage()
  const initialPage = useMemo(
    () => ({ items: data?.recipes ?? [], hasMore: data?.hasMore ?? false }),
    [data]
  )
  const {
    items: recipes,
    hasMore,
    loading: loadingMore,
    loadMore
  } = usePaginatedList(initialPage, fetchPage, (a, b) => a.id === b.id)

  return (
    <PageContainer>
      <PageHeader
        title="Recipes"
        actions={
          <Button asChild>
            <Link href="/recipes/new">New Recipe</Link>
          </Button>
        }
      />

      {isLoading && <LoadingState label="recipes" />}
      {error && <ErrorState what="recipes" />}
      {data && recipes.length === 0 && (
        <EmptyState>No recipes yet. Create your first one!</EmptyState>
      )}
      {data && recipes.length > 0 && (
        <>
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {recipes.map((recipe) => (
              <RecipeCard key={recipe.id} recipe={recipe} />
            ))}
          </div>
          {hasMore && <LoadMoreButton onClick={loadMore} loading={loadingMore} />}
        </>
      )}
    </PageContainer>
  )
}
