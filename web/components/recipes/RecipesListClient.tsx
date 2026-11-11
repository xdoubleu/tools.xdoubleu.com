import { useMemo } from 'react'
import { Link } from '@/components/ui/link'
import { useRecipes, useFetchRecipesPage } from '@/hooks/useRecipes'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { LinkCard } from '@/components/ui/link-card'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function RecipeCard({ recipe }: { recipe: Recipe }) {
  return (
    <LinkCard href={`/recipes/${recipe.id}`} linkClassName="p-4">
      <p className="break-words font-medium">{recipe.name}</p>
      <p className="text-sm text-muted mt-1">
        Serves {recipe.baseServings}
        {recipe.isDraft && (
          <Badge variant="secondary" className="ml-2">
            Draft
          </Badge>
        )}
      </p>
    </LinkCard>
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
            <Link href="/recipes/new">New recipe</Link>
          </Button>
        }
      />

      {isLoading && <LoadingState placement="page" label="recipes" />}
      {error && <ErrorState placement="page" what="recipes" />}
      {data && recipes.length === 0 && (
        <EmptyState placement="page">No recipes yet. Create your first one!</EmptyState>
      )}
      {data && recipes.length > 0 && (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
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
