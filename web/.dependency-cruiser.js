/**
 * dependency-cruiser config for web/.
 *
 * Two forbidden rules:
 *   - no-cross-domain-imports: app/<domain>/ and components/<domain>/ may only
 *     reach their own domain's components/lib, not another domain's. "Own
 *     domain" falls out of a capture group ($1) rather than being enumerated
 *     per domain, so a new domain is covered automatically.
 *   - no-react-in-lib: lib/** (excluding lib/gen/, the generated ConnectRPC
 *     clients) must not import react/react-dom — the checkable shape of
 *     docs/convention-ui-standards.md's "server/client import trap": lib/
 *     must stay framework-agnostic so a Server Component can safely import it
 *     for an unrelated constant.
 *
 * Pre-existing violations of both rules were found on the full-repo baseline
 * run (issue #1630) and are allow-listed below by name, each pointing at the
 * follow-up issue that tracks fixing it, rather than silently grandfathered.
 */

// Baseline exemption for no-cross-domain-imports: components/recipes/ and
// lib/recipes/ hold several files actually owned by the mealplans/
// shoppinglist domains (they use those domains' hooks/gen clients, not
// recipes'). Moving them is tracked in #1730 rather than done inline here.
// Remove this list (and let the rule cover these paths again) once #1730
// lands.
const BASELINE_EXEMPT_FROM_DOMAIN_ISOLATION = [
  '^app/mealplans/',
  '^app/shoppinglist/',
  '^components/shoppinglist/'
]

module.exports = {
  forbidden: [
    {
      name: 'no-cross-domain-imports',
      comment:
        "app/<domain>/ and components/<domain>/ must not import another domain's components/<domain>/ or lib/<domain>/. dashboard is exempt in the `from` direction only (ADR-0007: it reaches other apps' UI directly, the frontend analog of reaching another app only through exported struct methods on the Go side). See #1730 for the one tracked, allow-listed exception to the `to` side.",
      severity: 'error',
      from: {
        path: '^(?:app|components)/(?!dashboard/)([^/]+)/',
        pathNot: BASELINE_EXEMPT_FROM_DOMAIN_ISOLATION
      },
      to: {
        // components/ui, components/notifications, lib/gen, lib/server, and
        // lib/oauth2as are shared, framework/infra folders, not
        // domain-owned ones:
        //   - components/notifications/NotificationToggleList.tsx is
        //     deliberately shared by the monitoring and feeds
        //     notification-settings pages (issue #1228, see its own
        //     doc-comment) — it has no `app/notifications` route of its own.
        //   - lib/oauth2as is used only by app/oauth (name mismatch is
        //     historical: it mirrors the api's own oauth2as package), so
        //     it's excluded here the same way gen/server are, rather than
        //     needing a $1-based exemption that would never match its
        //     differently-spelled folder.
        path: '^(?:components|lib)/(?!ui/|notifications/|gen/|server/|oauth2as/)([^/]+)/',
        pathNot: ['^components/$1/', '^lib/$1/']
      }
    },
    {
      name: 'no-react-in-lib',
      comment:
        "lib/** (excluding lib/gen/) must stay framework-agnostic — no react/react-dom import — so a Server Component can safely import from lib/ for an unrelated constant (docs/convention-ui-standards.md's server/client import trap). #1731 tracks moving the two current violators (real React hooks living under lib/) into hooks/.",
      severity: 'error',
      from: {
        path: '^lib/',
        pathNot: [
          '^lib/gen/',
          // lib/server/ is the RSC-only ConnectRPC transport (web/CLAUDE.md's
          // "Data Flow (RSC + SWR)"), and its only React import is
          // `cache` from 'react' — a server-safe API valid inside Server
          // Components, not one of the client-only hooks
          // (useState/useEffect/etc.) this rule targets. Excluding it here
          // is a deliberate narrowing of the rule to its documented intent,
          // not a grandfathered violation.
          '^lib/server/',
          // #1731: real React hooks (useState/useEffect/useRef/useCallback)
          // living under lib/ instead of hooks/. Remove these two lines once
          // #1731 moves them into hooks/.
          '^lib/progressSocket\\.ts$',
          '^lib/trains/journeySocket\\.ts$'
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
