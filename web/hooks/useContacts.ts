import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { ContactsService } from '@/lib/gen/contacts/v1/contacts_connect'
import type { ListContactsResponse } from '@/lib/gen/contacts/v1/contacts_pb'

export function useContacts() {
  const client = createServiceClient(ContactsService)
  return useSWR<ListContactsResponse, Error>('/contacts', () => client.listContacts({}))
}

export function useCreateContact() {
  const client = createServiceClient(ContactsService)
  return (email: string, displayName: string) => client.createContact({ email, displayName })
}

export function useAcceptContact() {
  const client = createServiceClient(ContactsService)
  return (id: string, displayName: string) => client.acceptContact({ id, displayName })
}

export function useDeclineContact() {
  const client = createServiceClient(ContactsService)
  return (id: string) => client.declineContact({ id })
}

export function useDeleteContact() {
  const client = createServiceClient(ContactsService)
  return (id: string) => client.deleteContact({ id })
}
