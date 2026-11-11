import CategoryManager from '@/components/shoppinglist/CategoryManager'
import ItemCatalog from '@/components/shoppinglist/ItemCatalog'
import StoreManager from '@/components/shoppinglist/StoreManager'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { SettingsSection, SettingsSections } from '@/components/ui/settings-section'

export default function ShoppingListSettingsPage() {
  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Shopping list settings"
        breadcrumb={[{ label: 'Shopping list', href: '/shoppinglist' }, { label: 'Settings' }]}
      />

      <SettingsSections>
        <SettingsSection
          title="Categories"
          description="Group items into aisles like Produce, Dairy or Bakery."
        >
          <CategoryManager />
        </SettingsSection>

        <SettingsSection
          title="Item catalog"
          description="Assign each item a category once. It applies to custom items and recipe ingredients everywhere."
        >
          <ItemCatalog />
        </SettingsSection>

        <SettingsSection
          title="Stores"
          description="Set the aisle order for each store. Exports follow the selected store’s order."
        >
          <StoreManager />
        </SettingsSection>
      </SettingsSections>
    </PageContainer>
  )
}
