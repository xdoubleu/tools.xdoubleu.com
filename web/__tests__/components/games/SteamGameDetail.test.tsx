import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { AchievementSchema, GameSchema } from '@/lib/gen/games/v1/games_pb'
import { GameAchievements, SteamGameHeader } from '@/components/games/SteamGameDetail'

const achievements = [
  create(AchievementSchema, {
    name: 'a',
    displayName: 'Common',
    achieved: true,
    globalPercent: 90
  }),
  create(AchievementSchema, { name: 'b', displayName: 'Rare', achieved: false, globalPercent: 5 }),
  create(AchievementSchema, { name: 'c', displayName: 'Unknown', achieved: false })
]

describe('SteamGameHeader', () => {
  it('renders the title, stats and a Delisted badge', () => {
    const game = create(GameSchema, {
      name: 'Old Game',
      playtime: 180,
      completionRate: '40',
      isDelisted: true
    })
    render(<SteamGameHeader game={game} meta={<span>extra</span>} />)

    expect(screen.getByRole('heading', { level: 1, name: 'Old Game' })).toBeInTheDocument()
    expect(screen.getByText('3 hrs played')).toBeInTheDocument()
    expect(screen.getByText('Completion: 40%')).toBeInTheDocument()
    expect(screen.getByText('Delisted')).toBeInTheDocument()
    expect(screen.getByText('extra')).toBeInTheDocument()
  })
})

describe('GameAchievements', () => {
  it('sorts by global percent and hides completed ones by default', () => {
    render(<GameAchievements achievements={achievements} showCompleted={false} />)

    expect(screen.getByRole('heading', { name: 'Achievements (1/3)' })).toBeInTheDocument()
    expect(screen.queryByText('Common')).not.toBeInTheDocument()
    const titles = screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)
    expect(titles).toEqual(['Rare', 'Unknown'])
  })

  it('shows completed ones when asked', () => {
    render(<GameAchievements achievements={achievements} showCompleted />)

    const titles = screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)
    expect(titles).toEqual(['Common', 'Rare', 'Unknown'])
  })

  it('says when every achievement is completed', () => {
    render(<GameAchievements achievements={[achievements[0]]} showCompleted={false} />)

    expect(screen.getByText('All achievements completed.')).toBeInTheDocument()
  })

  it('says when the game has no achievements', () => {
    render(<GameAchievements achievements={[]} showCompleted={false} />)

    expect(screen.getByText('No achievements for this game.')).toBeInTheDocument()
  })
})
