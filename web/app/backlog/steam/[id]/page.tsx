import SteamGameClient from './SteamGameClient'

export default async function SteamGamePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  return <SteamGameClient id={id} />
}
