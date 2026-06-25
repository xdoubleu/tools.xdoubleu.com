// Jest manual mock for react-markdown. The real library is ESM-only and
// cannot run under Jest's CommonJS transform. In tests we just render children
// as plain text inside a div so description content is still assertable.
export default function ReactMarkdown({ children }: { children: string }) {
  return <div>{children}</div>
}
