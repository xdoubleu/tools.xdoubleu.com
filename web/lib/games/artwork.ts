const CAPSULE = 'capsule_616x353.jpg'

/** The app's header image, which exists for apps that have no capsule. */
export function headerFallback(imageUrl: string): string {
  return imageUrl.includes(CAPSULE)
    ? imageUrl
        .replace('store_item_assets/steam/apps/', 'steam/apps/')
        .replace(CAPSULE, 'header.jpg')
    : ''
}
