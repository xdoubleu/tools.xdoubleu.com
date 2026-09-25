// react-markdown is ESM-only; render children as plain text.
export default function ReactMarkdown({ children }: { children: string }) {
  return <div>{children}</div>
}
