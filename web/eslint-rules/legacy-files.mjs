// Files exempt from the ui/* rules until their domain is migrated.
// Each domain PR deletes its section; delete this file once it is empty.
// Entries are globs, so route brackets are escaped.
export const legacyUiFiles = [
  // recipes, mealplans, shoppinglist
  'app/mealplans/\\[id\\]/MealPlanClient.tsx',
  'app/mealplans/\\[id\\]/edit/EditPlanClient.tsx',
  'app/recipes/\\[id\\]/RecipeClient.tsx',
  'app/recipes/\\[id\\]/edit/EditRecipeClient.tsx',
  'app/recipes/new/page.tsx',
  'app/shoppinglist/settings/page.tsx',
  'components/mealplans/MealPlanCalendar.tsx',
  'components/mealplans/MealPlanEntryForm.tsx',
  'components/mealplans/MealPlanMealChip.tsx',
  'components/mealplans/MealPlanWeekGrid.tsx',
  'components/mealplans/PlanForm.tsx',
  'components/mealplans/PlansListClient.tsx',
  'components/recipes/RecipeForm.tsx',
  'components/recipes/RecipesListClient.tsx',
  'components/shoppinglist/AddItemForm.tsx',
  'components/shoppinglist/CategoryManager.tsx',
  'components/shoppinglist/ExportDialog.tsx',
  'components/shoppinglist/ItemCatalog.tsx',
  'components/shoppinglist/MealPlanGroupFilter.tsx',
  'components/shoppinglist/MealPlanItemsPreview.tsx',
  'components/shoppinglist/ShoppingList.tsx',
  'components/shoppinglist/ShoppingListPageClient.tsx',
  'components/shoppinglist/StoreManager.tsx',
  // trains + watchparty
  'app/watchparty/\\[id\\]/ViewerClient.tsx',
  'app/watchparty/\\[id\\]/presenter/PresenterClient.tsx',
  'app/watchparty/page.tsx',
  'components/trains/JourneyAlternativePanel.tsx',
  'components/trains/JourneyDetailClient.tsx',
  'components/trains/JourneyLegCard.tsx',
  'components/trains/JourneyResults.tsx',
  'components/trains/SavedCommutes.tsx',
  'components/trains/StationField.tsx',
  'components/trains/TrainsClient.tsx'
]
