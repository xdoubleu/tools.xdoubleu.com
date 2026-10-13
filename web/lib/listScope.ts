/** Query param naming the list (solo or family-shared) a movies or taste page shows. */
export const LIST_PARAM = 'list'
const SHARED = 'shared'

/** Whether the page shows the family's shared list rather than the solo one. */
export function isSharedList(param: string | null | undefined): boolean {
  return param === SHARED
}

/** The path, keeping the shared list when it is the one being viewed. */
export function listHref(path: string, shared: boolean): string {
  return shared ? `${path}?${LIST_PARAM}=${SHARED}` : path
}

/** The request field for a list: set only for the shared one. */
export function sharedField(shared: boolean): { shared?: true } {
  return shared ? { shared: true } : {}
}
