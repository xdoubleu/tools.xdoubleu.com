import type { Module, QuizQuestion } from '@/lib/gen/learningpaths/v1/learningpaths_pb'

// QuizSelection is one selected option index per question; undefined until
// answered. Quizzes are auto-checked against the authored correct index, so
// this is the only state the take-quiz surface needs to keep.
export type QuizSelection = Array<number | undefined>

// The module's authored quiz questions; undefined when the module has no quiz.
// The module owns a single quiz as a flat list, per the proto contract.
export function moduleQuiz(module: Module): QuizQuestion[] | undefined {
  return module.quiz.length > 0 ? module.quiz : undefined
}

// The terminal checkpoint item carrying a module's quiz, by id; undefined
// when the module has no quiz. Passing the quiz completes this item.
export function quizCheckpointItemId(module: Module): string | undefined {
  if (!moduleQuiz(module)) return undefined
  const checkpoints = module.items.filter((item) => item.type === 'checkpoint')
  const terminal =
    checkpoints.length > 0
      ? checkpoints[checkpoints.length - 1]
      : module.items[module.items.length - 1]
  return terminal?.id
}

// isAnswerCorrect checks one selection against the authored correct index.
export function isAnswerCorrect(
  question: QuizQuestion,
  selectedIndex: number | undefined
): boolean {
  return selectedIndex !== undefined && selectedIndex === question.correctAnswerIndex
}

// percentCorrect returns the rounded percent of questions the selection gets
// right; unanswered questions count as wrong.
export function percentCorrect(questions: QuizQuestion[], selection: QuizSelection): number {
  if (questions.length === 0) return 0
  const correct = questions.filter((question, i) => isAnswerCorrect(question, selection[i])).length
  return Math.round((correct / questions.length) * 100)
}

// The pass threshold for the module quiz, expressed as a percent of questions
// that must be answered correctly. The proto model carries no per-quiz
// threshold, so the gate uses a single constant here.
export const QUIZ_PASS_THRESHOLD_PERCENT = 100

// didPass decides whether the attempt clears the pass threshold. A quiz with
// no questions never passes.
export function didPass(questions: QuizQuestion[], selection: QuizSelection): boolean {
  if (questions.length === 0) return false
  return percentCorrect(questions, selection) >= QUIZ_PASS_THRESHOLD_PERCENT
}
