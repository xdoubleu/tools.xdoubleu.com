/** Case-insensitive name clash, as the API's unique indexes enforce. */
export function hasName(
  list: { id: string; name: string }[],
  name: string,
  exceptId = ''
): boolean {
  const lower = name.toLowerCase()
  return list.some((e) => e.id !== exceptId && e.name.toLowerCase() === lower)
}
