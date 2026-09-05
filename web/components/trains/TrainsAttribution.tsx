'use client'

import { useTrainsFeedInfo } from '@/hooks/useTrains'

/**
 * Required CC BY 4.0 attribution for the NMBS-SNCB open-data feed (issue
 * #1389), shown on this page because it's the first one that displays the
 * data. The dataset date comes from the feed's own feed_info.feed_version
 * so it can't go stale.
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
