// Files exempt from the ui/* rules until their domain is migrated.
// Each domain PR deletes its section; delete this file once it is empty.
// Entries are globs, so route brackets are escaped.
export const legacyUiFiles = [
  // trains + watchparty
  'app/watchparty/\\[id\\]/ViewerClient.tsx',
  'app/watchparty/\\[id\\]/presenter/PresenterClient.tsx',
  'app/watchparty/page.tsx',
  'components/trains/JourneyAlternativePanel.tsx',
  'components/trains/JourneyDetailClient.tsx',
  'components/trains/JourneyLegCard.tsx',
  'components/trains/JourneyResults.tsx',
  'components/trains/SavedCommutes.tsx',
  'components/trains/StationField.tsx',
  'components/trains/TrainsClient.tsx'
]
