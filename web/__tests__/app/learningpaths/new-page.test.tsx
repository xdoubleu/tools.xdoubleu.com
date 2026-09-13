import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

const mockPush = jest.fn()

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush })
}))

jest.mock('@/components/learningpaths/LearningPathForm', () => ({
  __esModule: true,
  default: ({ onSave, onCancel }: { onSave: (id: string) => void; onCancel: () => void }) => (
    <div data-testid="learning-path-form">
      <button onClick={() => onSave('new-id')}>save</button>
      <button onClick={onCancel}>cancel</button>
    </div>
  )
}))

import NewLearningPathPage from '@/app/learningpaths/new/page'

beforeEach(() => jest.clearAllMocks())

describe('NewLearningPathPage', () => {
  it('renders the heading and form', () => {
    render(<NewLearningPathPage />)
    expect(screen.getByRole('heading', { name: 'New Learning Path' })).toBeInTheDocument()
    expect(screen.getByTestId('learning-path-form')).toBeInTheDocument()
  })

  it('navigates to the new path on save', () => {
    render(<NewLearningPathPage />)
    fireEvent.click(screen.getByText('save'))
    expect(mockPush).toHaveBeenCalledWith('/learningpaths/new-id')
  })

  it('navigates back to the list on cancel', () => {
    render(<NewLearningPathPage />)
    fireEvent.click(screen.getByText('cancel'))
    expect(mockPush).toHaveBeenCalledWith('/learningpaths/list')
  })
})
