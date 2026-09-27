import React from 'react'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  ModuleSchema,
  ItemSchema,
  QuizQuestionSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import ModuleQuiz from '@/components/learningpaths/ModuleQuiz'

function moduleWithQuiz() {
  return create(ModuleSchema, {
    id: 'm1',
    title: 'Module',
    items: [
      create(ItemSchema, { id: 'i1', type: 'read', description: 'read', completed: true }),
      create(ItemSchema, { id: 'c1', type: 'checkpoint', description: 'quiz', completed: false })
    ],
    quiz: [
      create(QuizQuestionSchema, {
        prompt: 'Q1',
        options: ['a', 'b', 'c'],
        correctAnswerIndex: 0
      }),
      create(QuizQuestionSchema, {
        prompt: 'Q2',
        options: ['x', 'y'],
        correctAnswerIndex: 1
      })
    ]
  })
}

function moduleWithoutQuiz() {
  return create(ModuleSchema, {
    id: 'm2',
    title: 'Plain',
    items: [create(ItemSchema, { id: 'i1', type: 'read', description: 'read', completed: false })]
  })
}

function click(button: HTMLElement) {
  fireEvent.click(button)
}

describe('ModuleQuiz', () => {
  it('renders nothing for a module without a quiz', () => {
    render(<ModuleQuiz module={moduleWithoutQuiz()} onPassed={jest.fn()} />)
    expect(screen.queryByText('Module quiz')).toBeNull()
  })

  it('shows the questions with options and a disabled check button until all answered', () => {
    render(<ModuleQuiz module={moduleWithQuiz()} onPassed={jest.fn()} />)
    expect(screen.getByText('1. Q1')).toBeInTheDocument()
    expect(screen.getByText('2. Q2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check answers' })).toBeDisabled()
  })

  it('shows right/wrong as answers are selected', () => {
    render(<ModuleQuiz module={moduleWithQuiz()} onPassed={jest.fn()} />)
    click(screen.getByRole('button', { name: /^a$/ }))
    expect(screen.getByText('Correct')).toBeInTheDocument()
    click(screen.getByRole('button', { name: /^x$/ }))
    expect(screen.getByText('Wrong')).toBeInTheDocument()
  })

  it('completes the checkpoint on a passing attempt', async () => {
    const onPassed = jest.fn()
    render(<ModuleQuiz module={moduleWithQuiz()} onPassed={onPassed} />)
    click(screen.getByRole('button', { name: /^a$/ }))
    click(screen.getByRole('button', { name: /^y$/ }))
    await act(async () => {
      click(screen.getByRole('button', { name: 'Check answers' }))
    })
    expect(onPassed).toHaveBeenCalledWith('c1')
    expect(screen.getByText('Passed — module complete')).toBeInTheDocument()
  })

  it('does not complete on a failing attempt and allows a retake', () => {
    const onPassed = jest.fn()
    render(<ModuleQuiz module={moduleWithQuiz()} onPassed={onPassed} />)
    click(screen.getByRole('button', { name: /^b$/ }))
    click(screen.getByRole('button', { name: /^y$/ }))
    click(screen.getByRole('button', { name: 'Check answers' }))
    expect(onPassed).not.toHaveBeenCalled()
    expect(screen.getByText(/Not quite — 50%/)).toBeInTheDocument()

    click(screen.getByRole('button', { name: 'Retake' }))
    expect(screen.queryByText(/Not quite/)).toBeNull()
    expect(screen.getByRole('button', { name: 'Check answers' })).toBeDisabled()
  })

  it('shows a completed summary when the checkpoint is already complete', () => {
    const mod = moduleWithQuiz()
    mod.items[1].completed = true
    render(<ModuleQuiz module={mod} onPassed={jest.fn()} />)
    expect(screen.getByText('Passed — module complete')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Check answers' })).toBeNull()
  })
})
