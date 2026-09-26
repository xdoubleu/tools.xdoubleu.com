'use client'

import { useState } from 'react'
import { useSteam, useSteamProgress } from '@/hooks/useGames'
import { StatTile } from '@/components/ui/stat'
import SteamDistributionChart from '@/components/games/SteamDistributionChart'
import SteamProgressChart from '@/components/games/SteamProgressChart'
import { Button } from '@/components/ui/button'
import { DateRangeFields } from '@/components/ui/date-range-fields'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { cn } from '@/lib/cn'
import { oneYearAgo, today } from '@/lib/dates'

// ponytail: panel width mirrors dialog.tsx's rightContentClass (`w-[calc(100%-3rem)] max-w-md`)
// so the close handle sits flush on the panel's left edge; keep the two in sync.
const PANEL_OFFSET = 'right-[min(calc(100%_-_3rem),28rem)]'

function GamesStatsPanelContent() {
  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())

  const { data: steamData, error: steamError, isLoading: steamLoading } = useSteam()
  const { data: progressData, isLoading: progressLoading } = useSteamProgress(
    progressStart,
    progressEnd
  )

  const steam = steamData?.steam
  const progressSteam = progressData?.steam
  const progressChartData =
    progressSteam?.labels?.map((label, idx) => ({
      label,
      value: parseFloat(progressSteam.values?.[idx] ?? '0')
    })) ?? []

  return (
    <>
      {steamLoading && <LoadingState label="stats" />}
      {steamError && <ErrorState what="Steam data" />}

      {steam && (
        <div className="flex flex-col gap-6">
          <div className="grid grid-cols-2 gap-3">
            <StatTile label="Total backlog" value={steam.totalBacklog} />
            <StatTile label="Current rate" value={`${steam.currentRate}%`} />
            <StatTile label="In progress" value={steam.inProgress.length} />
            <StatTile label="Completed" value={steam.completed.length} />
          </div>

          <div>
            <h3 className="mb-2 text-sm font-semibold">Distribution</h3>
            <div className="h-64 w-full">
              <SteamDistributionChart distribution={steam.distribution} />
            </div>
          </div>

          <div>
            <div className="mb-2 flex flex-wrap items-end justify-between gap-3">
              <h3 className="text-sm font-semibold">Progress</h3>
              <DateRangeFields
                idPrefix="panel"
                start={progressStart}
                onStartChange={setProgressStart}
                end={progressEnd}
                onEndChange={setProgressEnd}
              />
            </div>
            {progressLoading && <LoadingState label="progress" />}
            {!progressLoading && progressChartData.length === 0 && (
              <p className="text-muted">No progress data for this range.</p>
            )}
            {progressChartData.length > 0 && (
              <div className="h-64 w-full">
                <SteamProgressChart data={progressChartData} />
              </div>
            )}
          </div>
        </div>
      )}
    </>
  )
}

export default function GamesStatsPanel() {
  const [open, setOpen] = useState(false)

  return (
    <>
      {!open && (
        <Button
          variant="secondary"
          size="sm"
          className="sm:fixed sm:right-[env(safe-area-inset-right)] sm:top-1/2 sm:z-40 sm:min-w-11 sm:-translate-y-1/2 sm:rounded-l-xl sm:rounded-r-none"
          aria-label="Open library stats"
          onClick={() => setOpen(true)}
        >
          {/* Inline in the page's button row on phones, where a fixed edge tab would cover content. */}
          <span className="sm:hidden">Library stats</span>
          <span className="hidden sm:inline">‹</span>
        </Button>
      )}
      <Dialog open={open} onOpenChange={setOpen} modal={false}>
        <DialogContent side="right" className="overflow-x-hidden">
          <DialogHeader>
            <DialogTitle>Library stats</DialogTitle>
          </DialogHeader>
          {open && <GamesStatsPanelContent />}
        </DialogContent>
      </Dialog>
      {open && (
        <Button
          variant="secondary"
          size="sm"
          className={cn(
            'fixed top-1/2 z-60 min-w-11 -translate-y-1/2 rounded-l-xl rounded-r-none',
            PANEL_OFFSET
          )}
          aria-label="Close library stats"
          onClick={() => setOpen(false)}
        >
          ›
        </Button>
      )}
    </>
  )
}
