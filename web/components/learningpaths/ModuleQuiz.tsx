'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/cn'
import type { Module, QuizQuestion } from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import {
  didPass,
  isAnswerCorrect,
  moduleQuiz,
  percentCorrect,
  QUIZ_PASS_THRESHOLD_PERCENT,
  quizCheckpointItemId,
  type QuizSelection
} from '@/lib/learningpaths/quiz'

export interface ModuleQuizProps {
  module: Module
  // onPassed marks the terminal checkpoint complete, which completes the
  // module. Called exactly once per passing attempt.
  onPassed: (checkpointItemId: string) => Promise<void> | void
}

/** The module's take-quiz surface: auto-checked, threshold-gated, retakes. */
export default function ModuleQuiz({ module, onPassed }: ModuleQuizProps) {
  const questions: QuizQuestion[] | undefined = moduleQuiz(module)
  const checkpointId = quizCheckpointItemId(module)
  const [selection, setSelection] = useState<QuizSelection>([])
  const [result, setResult] = useState<'pending' | 'passed' | 'failed'>('pending')
  const [submitting, setSubmitting] = useState(false)

  if (!questions || !checkpointId) return null

  const alreadyCompleted = module.items.some((item) => item.id === checkpointId && item.completed)
  if (alreadyCompleted) {
    return (
      <div className="mb-6">
        <h3 className="text-sm font-semibold mb-2">Module quiz</h3>
        <Badge variant="success">Passed — module complete</Badge>
      </div>
    )
  }

  const allAnswered = selection.filter((s) => s !== undefined).length === questions.length

  const setAnswer = (questionIndex: number, optionIndex: number) => {
    const next = [...selection]
    next[questionIndex] = optionIndex
    setSelection(next)
    setResult('pending')
  }

  const submit = async () => {
    if (didPass(questions, selection)) {
      setResult('passed')
      setSubmitting(true)
      try {
        await onPassed(checkpointId)
      } finally {
        setSubmitting(false)
      }
      return
    }
    setResult('failed')
  }

  const retake = () => {
    setSelection([])
    setResult('pending')
  }

  return (
    <div className="mb-6">
      <h3 className="text-sm font-semibold mb-2">
        Module quiz — pass {QUIZ_PASS_THRESHOLD_PERCENT}% to complete
      </h3>

      {questions.map((question, q) => {
        const answered = selection[q] !== undefined
        const correct = isAnswerCorrect(question, selection[q])
        return (
          <div key={q} className="mb-3">
            <p className="text-sm font-medium">
              {q + 1}. {question.prompt}
            </p>
            <div className="flex flex-col gap-1.5 mt-2">
              {question.options.map((option, o) => {
                const chosen = selection[q] === o
                const variant = !chosen ? 'secondary' : correct ? 'default' : 'destructive'
                return (
                  <Button
                    key={o}
                    variant={variant}
                    size="sm"
                    className="justify-start text-left w-full"
                    disabled={result === 'passed'}
                    onClick={() => setAnswer(q, o)}
                  >
                    {option}
                    {chosen ? (correct ? ' ✓' : ' ✗') : ''}
                  </Button>
                )
              })}
            </div>
            {answered && (
              <Badge variant={correct ? 'success' : 'danger'} className="mt-1.5">
                {correct ? 'Correct' : 'Wrong'}
              </Badge>
            )}
          </div>
        )
      })}

      {result === 'failed' && (
        <div className="flex items-center gap-2 mb-3">
          <Badge variant="warn">
            Not quite — {percentCorrect(questions, selection)}% (need {QUIZ_PASS_THRESHOLD_PERCENT}%)
          </Badge>
          <Button variant="secondary" size="sm" onClick={retake}>
            Retake
          </Button>
        </div>
      )}

      {result === 'passed' && <Badge variant="success">Passed — module complete</Badge>}

      <Button
        variant="default"
        disabled={!allAnswered || result === 'passed' || submitting}
        className={cn(result === 'failed' && 'mt-0 mb-0')}
        onClick={submit}
      >
        {submitting ? 'Checking…' : 'Check answers'}
      </Button>
    </div>
  )
}