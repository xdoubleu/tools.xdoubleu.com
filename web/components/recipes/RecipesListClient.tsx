import { useMemo, useState } from 'react'
import { ChefHat } from 'lucide-react'
import { Link } from '@/components/ui/link'
import { useRecipes, useFetchRecipesPage } from '@/hooks/useRecipes'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { CollectionGrid } from '@/components/ui/collection-grid'
import { CollectionToolbar } from '@/components/ui/collection-toolbar'
import { ItemCard } from '@/components/ui/item-card'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function RecipeCard({ recipe }: { recipe: Recipe }) {
  return (
    <ItemCard
      href={`/recipes/${recipe.id}`}
      title={recipe.name}
      meta={`Serves ${recipe.baseServings}`}
      status={recipe.isDraft ? <Badge variant="secondary">Draft</Badge> : undefined}
    />
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

  const [query, setQuery] = useState('')
  const [draftsOnly, setDraftsOnly] = useState(false)
  const shown = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return recipes.filter(
      (r) => (!draftsOnly || r.isDraft) && r.name.toLowerCase().includes(needle)
    )
  }, [recipes, query, draftsOnly])

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
        <EmptyState
          placement="page"
          icon={ChefHat}
          action={
            <Button asChild>
              <Link href="/recipes/new">New recipe</Link>
            </Button>
          }
        >
          No recipes yet. Create your first one!
        </EmptyState>
      )}
      {data && recipes.length > 0 && (
        <div className="space-y-4">
          <CollectionToolbar
            noun="recipes"
            query={query}
            onQueryChange={setQuery}
            facets={[
              {
                // Stryker disable next-line StringLiteral: the id is only a React key
                id: 'drafts',
                label: 'Drafts',
                active: draftsOnly,
                onToggle: () => setDraftsOnly((on) => !on)
              }
            ]}
            count={shown.length}
          />
          {shown.length === 0 ? (
            <EmptyState placement="inline">No recipes match.</EmptyState>
          ) : (
            <CollectionGrid>
              {shown.map((recipe) => (
                <RecipeCard key={recipe.id} recipe={recipe} />
              ))}
            </CollectionGrid>
          )}
          {hasMore && <LoadMoreButton onClick={loadMore} loading={loadingMore} />}
        </div>
      )}
    </PageContainer>
  )
}
