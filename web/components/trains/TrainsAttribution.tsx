'use client'

import { useTrainsFeedInfo } from '@/hooks/useTrains'

/**
 * Required CC BY 4.0 attribution for the NMBS-SNCB feed, dated from the
 * feed's own feed_version.
 */
export default function TrainsAttribution() {
  const { data } = useTrainsFeedInfo()
  const feedVersion = data?.feedVersion

  return (
    <p className="text-xs text-muted">
      Source: NMBS-SNCB - Open Data{feedVersion ? ` - ${feedVersion}` : ''}. Contains data
      originally published by NMBS-SNCB, modified by tools.xdoubleu.com.
    </p>
  )
}
