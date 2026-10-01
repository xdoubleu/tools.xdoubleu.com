import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { cn } from '@/lib/cn'

interface BookDescriptionProps {
  description: string
  id?: string
  className?: string
}

/** A book's Markdown description. */
export default function BookDescription({ description, id, className }: BookDescriptionProps) {
  return (
    <div id={id} className={cn('prose prose-sm max-w-none text-fg', className)}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{description}</ReactMarkdown>
    </div>
  )
}
