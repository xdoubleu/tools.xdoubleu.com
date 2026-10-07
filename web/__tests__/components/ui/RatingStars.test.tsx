import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'

import { RatingStars } from '@/components/ui/rating-stars'

describe('RatingStars', () => {
  it('shows a read-only rating as an image', () => {
    render(<RatingStars value={2} size="md" />)
    expect(screen.queryAllByRole('button')).toHaveLength(0)
    const img = screen.getByRole('img', { name: '2 out of 5 stars' })
    expect(img).toHaveClass('inline-flex', 'text-lg')
    const glyphs = img.querySelectorAll('span')
    expect(glyphs).toHaveLength(5)
    expect(glyphs[1]).toHaveClass('text-star')
    expect(glyphs[2]).toHaveClass('text-border')
  })

  it('labels an unrated read-only display', () => {
    render(<RatingStars value={0} />)
    expect(screen.getByRole('img', { name: 'No rating' })).toHaveClass('text-sm')
  })

  it('rates, and clears when the current rating is clicked', () => {
    const onChange = jest.fn()
    render(<RatingStars value={3} onChange={onChange} />)
    expect(screen.getByLabelText('3 out of 5 stars')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Rate 1 star' }))
    expect(onChange).toHaveBeenLastCalledWith(1)
    fireEvent.click(screen.getByRole('button', { name: 'Rate 3 stars' }))
    expect(onChange).toHaveBeenLastCalledWith(0)
  })

  it('previews the hovered rating until the pointer leaves', () => {
    render(<RatingStars value={1} onChange={jest.fn()} size="md" />)
    const fifth = screen.getByRole('button', { name: 'Rate 5 stars' })
    expect(fifth).toHaveClass('text-border', 'text-xl')

    fireEvent.mouseEnter(fifth)
    expect(fifth).toHaveClass('text-star')
    fireEvent.mouseLeave(screen.getByLabelText('1 out of 5 stars'))
    expect(fifth).toHaveClass('text-border')
    expect(screen.getByRole('button', { name: 'Rate 1 star' })).toHaveClass('text-star')
  })

  it('drops a tapped preview so a cleared rating shows empty', () => {
    const { rerender } = render(<RatingStars value={3} onChange={jest.fn()} />)
    const third = screen.getByRole('button', { name: 'Rate 3 stars' })
    fireEvent.mouseEnter(third)
    fireEvent.click(third)
    rerender(<RatingStars value={0} onChange={jest.fn()} />)
    expect(third).toHaveClass('text-border')
    expect(screen.getByRole('group', { name: 'No rating' })).toBeInTheDocument()
  })

  it('locks its stars while disabled', () => {
    render(<RatingStars value={0} onChange={jest.fn()} disabled />)
    for (const b of screen.getAllByRole('button')) expect(b).toBeDisabled()
  })

  it('is enabled by default', () => {
    render(<RatingStars value={0} onChange={jest.fn()} />)
    const star = screen.getByRole('button', { name: 'Rate 2 stars' })
    expect(star).toBeEnabled()
    expect(star).toHaveClass('leading-none', 'hover:text-star')
    expect(star).not.toHaveClass('text-xl')
    expect(screen.getByLabelText('No rating')).toBeInTheDocument()
  })
})
