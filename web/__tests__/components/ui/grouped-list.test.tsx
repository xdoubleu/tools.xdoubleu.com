import { render, screen } from '@testing-library/react'
import { GroupedList, GroupedListItem } from '@/components/ui/grouped-list'

describe('GroupedList', () => {
  it('renders rows on one divided surface', () => {
    render(
      <GroupedList aria-label="Repos">
        <GroupedListItem>One</GroupedListItem>
        <GroupedListItem className="extra">Two</GroupedListItem>
      </GroupedList>
    )
    const list = screen.getByRole('list', { name: 'Repos' })
    expect(list).toHaveClass('divide-y', 'rounded-2xl', 'bg-card')
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
    expect(screen.getByText('Two')).toHaveClass('extra', 'px-4')
  })
})
