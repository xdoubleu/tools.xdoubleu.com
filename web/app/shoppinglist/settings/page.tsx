'use client'

import CategoryManager from '@/components/recipes/CategoryManager'
import ItemCatalog from '@/components/recipes/ItemCatalog'
import StoreManager from '@/components/recipes/StoreManager'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default function ShoppingListSettingsPage() {
  return (
    <PageContainer className="max-w-3xl p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Shopping List', href: '/shoppinglist' }, { label: 'Settings' }]}
      />
      <h1 className="mb-6 text-3xl font-bold">Shopping List Settings</h1>

      <div className="space-y-6">
        <Card>
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

        <Card>
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

        <Card>
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
