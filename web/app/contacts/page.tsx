import ContactsPageClient from '@/components/contacts/ContactsPageClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ContactsService } from '@/lib/gen/contacts/v1/contacts_pb'

export default async function ContactsPage() {
  const client = await createServerClient(ContactsService)
  const contacts = await fetchOrNull(() => client.listContacts({}))

  return (
    <SWRFallback fallback={contacts ? { [swrKeys.contacts]: contacts } : {}}>
      <ContactsPageClient />
    </SWRFallback>
  )
}
