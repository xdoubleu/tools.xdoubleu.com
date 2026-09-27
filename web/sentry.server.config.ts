import * as Sentry from '@sentry/nextjs'

Sentry.init({
  dsn: process.env.SENTRY_DSN_WEB,
  release: process.env.NEXT_PUBLIC_RELEASE || 'dev',
  tracesSampleRate: 1.0,
  ignoreErrors: ['The destination stream closed early.'],
  // A synthetic span summing chunk loads across requests; its duration isn't
  // attributable to the request it lands on.
  ignoreSpans: ['NextNodeServer.clientComponentLoading'],
  // v11 defaults to collecting everything; keep the restrictive v10 baseline.
  dataCollection: {
    userInfo: false,
    cookies: false,
    httpHeaders: {
      request: { deny: ['forwarded', '-ip', 'remote-', 'via', '-user'] },
      response: { deny: ['forwarded', '-ip', 'remote-', 'via', '-user'] }
    },
    httpBodies: [],
    urlQueryParams: { deny: ['forwarded', '-ip', 'remote-', 'via', '-user'] },
    genAI: { inputs: false, outputs: false },
    databaseQueryData: false,
    graphQL: { document: false, variables: false }
  }
})
