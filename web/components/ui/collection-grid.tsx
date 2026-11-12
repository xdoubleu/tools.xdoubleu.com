import { type ReactNode } from 'react'

/** The responsive grid of a collection's `ItemCard`s: one column on phones, two, then three. */
function CollectionGrid({ children }: { children: ReactNode }) {
  return <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">{children}</div>
}

export { CollectionGrid }
