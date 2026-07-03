import SteamDistributionClient from './SteamDistributionClient'

export default async function Page({ params }: { params: Promise<{ bucket: string }> }) {
  const { bucket } = await params
  return <SteamDistributionClient bucket={bucket} />
}
