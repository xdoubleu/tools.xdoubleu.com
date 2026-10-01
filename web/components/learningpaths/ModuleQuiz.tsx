'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { Module, QuizQuestion } from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import {
  didPass,
  isAnswerCorrect,
  isQuizPassed,
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

/** The module's take-quiz surface: graded on submit, threshold-gated, retakes. */
export default function ModuleQuiz({ module, onPassed }: ModuleQuizProps) {
  const questions: QuizQuestion[] | undefined = moduleQuiz(module)
  const checkpointId = quizCheckpointItemId(module)
  const [selection, setSelection] = useState<QuizSelection>([])
  const [result, setResult] = useState<'pending' | 'passed' | 'failed'>('pending')
  const [submitting, setSubmitting] = useState(false)

  if (!questions || !checkpointId) return null

  if (isQuizPassed(module) && result === 'pending') {
    return <Badge variant="success">Passed — module complete</Badge>
  }

  const graded = result !== 'pending'
  const allAnswered = selection.filter((s) => s !== undefined).length === questions.length

  const setAnswer = (questionIndex: number, optionIndex: number) => {
    const next = [...selection]
    next[questionIndex] = optionIndex
    setSelection(next)
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
    <div>
      <p className="mb-4 text-sm text-muted">
        {questions.length} questions — answer {QUIZ_PASS_THRESHOLD_PERCENT}% correctly to complete
        the module.
      </p>

      {questions.map((question, q) => {
        const correct = isAnswerCorrect(question, selection[q])
        return (
          <div key={q} className="mb-5">
            <p className="text-sm font-medium">
              {q + 1}. {question.prompt}
            </p>
            <div className="mt-2 flex flex-col gap-2">
              {question.options.map((option, o) => {
                const chosen = selection[q] === o
                const variant = !chosen
                  ? 'secondary'
                  : !graded || correct
                    ? 'default'
                    : 'destructive'
                return (
                  <Button
                    key={o}
                    variant={variant}
                    size="sm"
                    className="h-auto min-h-11 w-full justify-start py-2 text-left sm:h-auto sm:min-h-8"
                    aria-pressed={chosen}
                    disabled={graded}
                    onClick={() => setAnswer(q, o)}
                  >
                    {option}
                    {graded && chosen ? (correct ? ' ✓' : ' ✗') : ''}
                  </Button>
                )
              })}
            </div>
            {graded && (
              <Badge variant={correct ? 'success' : 'danger'} className="mt-1.5">
                {correct ? 'Correct' : 'Wrong'}
              </Badge>
            )}
          </div>
        )
      })}

      {result === 'failed' && (
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="warn">
            Not quite — {percentCorrect(questions, selection)}% (need {QUIZ_PASS_THRESHOLD_PERCENT}
            %)
          </Badge>
          <Button variant="secondary" size="sm" onClick={retake}>
            Retake
          </Button>
        </div>
      )}

      {result === 'passed' && !submitting && (
        <Badge variant="success">Passed — module complete</Badge>
      )}

      {(result === 'pending' || submitting) && (
        <Button variant="default" disabled={!allAnswered || graded} onClick={submit}>
          {submitting ? 'Saving…' : 'Check answers'}
        </Button>
      )}
    </div>
  )
}
