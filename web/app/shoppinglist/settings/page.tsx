import CategoryManager from '@/components/shoppinglist/CategoryManager'
import ItemCatalog from '@/components/shoppinglist/ItemCatalog'
import StoreManager from '@/components/shoppinglist/StoreManager'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function ShoppingListSettingsPage() {
  return (
    <PageContainer size="form">
      <PageHeader
        title="Shopping List Settings"
        breadcrumb={[{ label: 'Shopping List', href: '/shoppinglist' }, { label: 'Settings' }]}
      />

      <div className="space-y-6">
        <Card padding="none">
          <CardHeader>
            <CardTitle>Categories</CardTitle>
            <CardDescription>
              Group items into aisles like Produce, Dairy or Bakery.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <CategoryManager />
          </CardContent>
        </Card>

        <Card padding="none">
          <CardHeader>
            <CardTitle>Item catalog</CardTitle>
            <CardDescription>
              Assign each item a category once. It applies to custom items and recipe ingredients
              everywhere.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ItemCatalog />
          </CardContent>
        </Card>

        <Card padding="none">
          <CardHeader>
            <CardTitle>Stores</CardTitle>
            <CardDescription>
              Set the aisle order for each store. Exports follow the selected store&apos;s order.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <StoreManager />
          </CardContent>
        </Card>
      </div>
    </PageContainer>
  )
}
