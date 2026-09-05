'use client'

import { useState } from 'react'
import StationField from '@/components/trains/StationField'
import JourneyResults from '@/components/trains/JourneyResults'
import TrainsAttribution from '@/components/trains/TrainsAttribution'
import { Button } from '@/components/ui/button'
import { DateInput } from '@/components/ui/date-input'
import { Input } from '@/components/ui/input'
import { TogglePill } from '@/components/ui/toggle-pill'
import { PageContainer } from '@/components/ui/page-container'
import { useJourneySearch, useTrainsFeedInfo } from '@/hooks/useTrains'

function nowParts(): { date: string; time: string } {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return {
    date: `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`,
    time: `${pad(now.getHours())}:${pad(now.getMinutes())}`
  }
}

function toRfc3339(date: string, time: string): string {
  if (!date || !time) return ''
  const local = new Date(`${date}T${time}:00`)
  if (Number.isNaN(local.getTime())) return ''
  return local.toISOString()
}

export default function TrainsClient() {
  const initial = nowParts()

  const [originStopId, setOriginStopId] = useState('')
  const [originQuery, setOriginQuery] = useState('')
  const [destStopId, setDestStopId] = useState('')
  const [destQuery, setDestQuery] = useState('')
  const [date, setDate] = useState(initial.date)
  const [time, setTime] = useState(initial.time)
  const [arriveBy, setArriveBy] = useState(false)

  const { data: feedInfo } = useTrainsFeedInfo()
  const requestTime = toRfc3339(date, time)
  const {
    data: journeysData,
    error,
    isLoading
  } = useJourneySearch(originStopId, destStopId, requestTime, arriveBy)

  const swap = () => {
    setOriginStopId(destStopId)
    setOriginQuery(destQuery)
    setDestStopId(originStopId)
    setDestQuery(originQuery)
  }

  return (
    <PageContainer size="narrow" className="p-6">
      <h1 className="mb-6 text-3xl font-bold">Trains</h1>

      <div className="space-y-4">
        <div className="flex items-end gap-2">
          <div className="flex-1 space-y-4">
            <StationField
              label="From"
              query={originQuery}
              onQueryChange={(text) => {
                setOriginQuery(text)
                setOriginStopId('')
              }}
              onSelectStation={(stopId, name) => {
                setOriginStopId(stopId)
                setOriginQuery(name)
              }}
              placeholder="Origin station"
              autoFocus
            />
            <StationField
              label="To"
              query={destQuery}
              onQueryChange={(text) => {
                setDestQuery(text)
                setDestStopId('')
              }}
              onSelectStation={(stopId, name) => {
                setDestStopId(stopId)
                setDestQuery(name)
              }}
              placeholder="Destination station"
            />
          </div>
          <Button
            type="button"
            variant="secondary"
            size="icon"
            aria-label="Swap origin and destination"
            onClick={swap}
          >
            ⇅
          </Button>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <TogglePill label="Depart at" active={!arriveBy} onClick={() => setArriveBy(false)} />
          <TogglePill label="Arrive by" active={arriveBy} onClick={() => setArriveBy(true)} />
        </div>

        <div className="flex gap-2">
          <DateInput value={date} onChange={setDate} className="flex-1" aria-label="Date" />
          <Input
            type="time"
            value={time}
            onChange={(e) => setTime(e.target.value)}
            aria-label="Time"
            className="w-32"
          />
        </div>
      </div>

      <div className="mt-6">
        <JourneyResults
          ready={originStopId !== '' && destStopId !== ''}
          isLoading={isLoading}
          error={error}
          feedImported={feedInfo ? feedInfo.feedVersion !== '' : true}
          journeys={journeysData?.journeys ?? []}
        />
      </div>

      <div className="mt-8">
        <TrainsAttribution />
      </div>
    </PageContainer>
  )
}
