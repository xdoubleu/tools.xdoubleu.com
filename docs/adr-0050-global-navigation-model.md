# ADR-0050: One global navigation, sidebar on wide screens and a tab bar on phones

- Status: Accepted
- Affects: `web/components/{AppShell,Sidebar,Navbar,AccountMenu}.tsx`, `web/lib/apps.ts`
- Builds on: [adr-0034](adr-0034-vite-spa-served-by-the-api.md)

## Context

Apps were reachable only through the home page, nothing showed which app you were in, and every page carried a Settings link and Sign out in its top bar. Each app grew its own sub-navigation.

## Decision

- **One global bar, driven by the app registry** (`web/lib/apps.ts`): from `lg` a left sidebar (Today, then the apps the user can open grouped as in the registry, admin last, current app highlighted); on phones a bottom tab bar (Today, three pinned apps, Apps) with `/apps` as the full launcher; a slim top bar shows the current app name and the account menu.
- **One account menu** (sidebar foot, top-right below `lg`): account settings, family, appearance, sign out. Sign out flushes the offline outbox first. The footer is gone; the release hash lives on the account settings page.
- **An app's sections are in-page tabs** (`SegmentedTabs`), not a second bar.
- **Shared public dashboards and sign-in pages render without the shell**; the book reader and watch party presenter hide the tab bar.

## Rejected

- **A tab bar per app**: two bars compete on a phone and each app invents its own.
- **Per-app navigation in the top bar**: the bar can't hold sections and the account actions on a narrow screen.

## Consequences

- Adding an app means adding a registry entry; the sidebar, home grid and favicon follow.
- `lg` is the one breakpoint between the two shells.
