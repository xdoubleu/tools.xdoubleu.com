/** Focuses the element with this id, e.g. the field an empty state points at. */
export function focusById(id: string): void {
  const el = document.getElementById(id)
  if (!el) return
  el.scrollIntoView?.({ block: 'center' })
  el.focus()
}
