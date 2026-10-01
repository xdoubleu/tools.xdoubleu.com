import { create } from '@bufbuild/protobuf'
import {
  ItemSchema,
  ModuleSchema,
  QuizQuestionSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import { didPass, isAnswerCorrect, isQuizPassed, percentCorrect } from '@/lib/learningpaths/quiz'

function questions(spec: Array<[correct: number, options: number]> = [[0, 2]]) {
  return spec.map(([correct, options]) =>
    create(QuizQuestionSchema, {
      prompt: 'q',
      options: ['a', 'b', 'c'].slice(0, options),
      correctAnswerIndex: correct
    })
  )
}

describe('quiz gating', () => {
  describe('isAnswerCorrect', () => {
    it('is true only when the selected index is the correct one', () => {
      const question = create(QuizQuestionSchema, {
        prompt: '1+1?',
        options: ['1', '2', '3'],
        correctAnswerIndex: 1
      })
      expect(isAnswerCorrect(question, 1)).toBe(true)
      expect(isAnswerCorrect(question, 0)).toBe(false)
      expect(isAnswerCorrect(question, undefined)).toBe(false)
    })
  })

  describe('percentCorrect', () => {
    it('returns 100 when every selection is right', () => {
      const q = questions([
        [0, 2],
        [1, 2]
      ])
      expect(percentCorrect(q, [0, 1])).toBe(100)
    })

    it('returns 0 when every selection is wrong', () => {
      const q = questions([
        [0, 2],
        [1, 2]
      ])
      expect(percentCorrect(q, [1, 0])).toBe(0)
    })

    it('rounds a partial score', () => {
      const q = questions([
        [0, 2],
        [1, 2],
        [0, 2]
      ])
      // 1/3 correct ≈ 33%
      expect(percentCorrect(q, [0, 0, 1])).toBe(33)
    })

    it('treats unanswered questions as wrong', () => {
      const q = questions([
        [0, 2],
        [1, 2]
      ])
      expect(percentCorrect(q, [0, undefined])).toBe(50)
    })

    it('returns 0 for a quiz with no questions', () => {
      expect(percentCorrect(questions([]), [])).toBe(0)
    })
  })

  describe('didPass', () => {
    it('passes at and above the threshold', () => {
      const q = questions([
        [0, 2],
        [1, 2]
      ])
      expect(didPass(q, [0, 1])).toBe(true)
    })

    it('fails below the threshold (an unanswered or wrong answer)', () => {
      const q = questions([
        [0, 2],
        [1, 2]
      ])
      expect(didPass(q, [0, 0])).toBe(false)
    })

    it('never passes a quiz with no questions', () => {
      expect(didPass(questions([]), [])).toBe(false)
    })
  })

  describe('isQuizPassed', () => {
    function quizModule(checkpointCompleted: boolean, withQuiz = true) {
      return create(ModuleSchema, {
        items: [
          create(ItemSchema, { id: 'i1', type: 'read', completed: true }),
          create(ItemSchema, { id: 'c1', type: 'checkpoint', completed: checkpointCompleted })
        ],
        quiz: withQuiz ? questions() : []
      })
    }

    it('is true once the quiz checkpoint is complete', () => {
      expect(isQuizPassed(quizModule(true))).toBe(true)
    })

    it('is false while the quiz checkpoint is open', () => {
      expect(isQuizPassed(quizModule(false))).toBe(false)
    })

    it('is false for a module without a quiz', () => {
      expect(isQuizPassed(quizModule(true, false))).toBe(false)
    })
  })
})
