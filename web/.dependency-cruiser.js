module.exports = {
  forbidden: [
    {
      name: 'no-cross-domain-imports',
      comment:
        "app/<domain>/ and components/<domain>/ must not import another domain's components/<domain>/ or lib/<domain>/. dashboard is exempt in the `from` direction only (ADR-0007: it reaches other apps' UI directly, the frontend analog of reaching another app only through exported struct methods on the Go side). #1730 moved every shoppinglist/mealplans component that used to be misplaced under components/recipes and lib/recipes into its own domain, so this rule no longer needs an allow-list.",
      severity: 'error',
      from: {
        path: '^(?:app|components)/(?!dashboard/)([^/]+)/'
      },
      to: {
        // Shared folders, not domains. lib/oauth2as belongs to app/oauth (mirrors
        // the api's package name).
        path: '^(?:components|lib)/(?!ui/|notifications/|gen/|server/|oauth2as/)([^/]+)/',
        pathNot: ['^components/$1/', '^lib/$1/']
      }
    },
    {
      name: 'no-react-in-lib',
      comment:
        "lib/** (excluding lib/gen/) must stay framework-agnostic — no react/react-dom import — so a Server Component can safely import from lib/ for an unrelated constant (docs/convention-ui-standards.md's server/client import trap). The two real React hooks that used to live under lib/ (#1731) moved into hooks/.",
      severity: 'error',
      from: {
        path: '^lib/',
        pathNot: [
          '^lib/gen/',
          // RSC-only transport; its only React import is the server-safe `cache`.
          '^lib/server/'
        ]
      },
      to: {
        path: '^node_modules/(react|react-dom)/'
      }
    }
  ],
  options: {
    doNotFollow: {
      path: 'node_modules'
    },
    tsPreCompilationDeps: true,
    tsConfig: {
      fileName: 'tsconfig.json'
    },
    enhancedResolveOptions: {
      exportsFields: ['exports'],
      conditionNames: ['import', 'require', 'node', 'default']
    }
  }
}
