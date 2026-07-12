'use client'

import { useState } from 'react'
import { useSteam, useSteamProgress } from '@/hooks/useGames'
import GamesStatCard from '@/components/games/GamesStatCard'
import SteamDistributionChart from '@/components/games/SteamDistributionChart'
import SteamProgressChart from '@/components/games/SteamProgressChart'
import { Button } from '@/components/ui/button'
import { DateInput } from '@/components/ui/date-input'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
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
      {steamLoading && <p className="text-muted">Loading stats…</p>}
      {steamError && <p className="text-danger">Failed to load Steam data.</p>}

      {steam && (
        <div className="flex flex-col gap-6">
          <div className="grid grid-cols-2 gap-3">
            <GamesStatCard label="Total backlog" value={steam.totalBacklog} />
            <GamesStatCard label="Current rate" value={`${steam.currentRate}%`} />
            <GamesStatCard label="In progress" value={steam.inProgress.length} />
            <GamesStatCard label="Completed" value={steam.completed.length} />
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
              <div className="flex gap-3">
                <div>
                  <label htmlFor="panel-from" className="mb-1 block text-xs text-muted">
                    From
                  </label>
                  <DateInput
                    id="panel-from"
                    value={progressStart}
                    onChange={setProgressStart}
                    className="h-9 w-36"
                  />
                </div>
                <div>
                  <label htmlFor="panel-to" className="mb-1 block text-xs text-muted">
                    To
                  </label>
                  <DateInput
                    id="panel-to"
                    value={progressEnd}
                    onChange={setProgressEnd}
                    className="h-9 w-36"
                  />
                </div>
              </div>
            </div>
            {progressLoading && <p className="text-muted">Loading progress…</p>}
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
          className="fixed right-0 top-1/2 z-40 -translate-y-1/2 rounded-l-xl rounded-r-none"
          aria-label="Open library stats"
          onClick={() => setOpen(true)}
        >
          ‹
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
          className={`fixed ${PANEL_OFFSET} top-1/2 z-60 -translate-y-1/2 rounded-l-xl rounded-r-none`}
          aria-label="Close library stats"
          onClick={() => setOpen(false)}
        >
          ›
        </Button>
      )}
    </>
  )
}
