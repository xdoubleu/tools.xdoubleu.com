// repo-guard — OpenCode port of .claude/settings.json's hooks: worktree edit
// guard, main ff-merge on setup, and idle nudges for unshipped work (reusing
// stop-check-unshipped-work.sh and its dedupe state) and `make lint/docs`.
// No `@opencode/plugin` import: this dir has no node_modules.
import { execFileSync } from "node:child_process"
import { join } from "node:path"

interface PermissionEvaluation {
  readonly sessionID: string
  readonly action: string
  readonly resources: readonly string[]
  effect: "allow" | "ask" | "deny"
  message?: string
}

interface PluginContext {
  readonly location: { readonly directory: string }
  readonly permission: {
    hook(
      name: "evaluate",
      callback: (event: PermissionEvaluation) => Promise<void> | void,
    ): Promise<{ dispose(): Promise<void> }>
  }
  readonly session: {
    hook(
      name: "prompt",
      callback: (event: { readonly sessionID: string }) => Promise<void> | void,
    ): Promise<{ dispose(): Promise<void> }>
    get(input: { sessionID: string }): Promise<{ location: { directory: string } }>
    wait(input: { sessionID: string }): Promise<unknown>
    synthetic(input: { sessionID: string; text: string }): Promise<unknown>
  }
}

const WORKTREE_MARKER = "/.claude/worktrees/"

function git(dir: string, ...args: string[]): string {
  return execFileSync("git", ["-C", dir, ...args], {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "ignore"],
  }).trim()
}

function inWorktree(dir: string): boolean {
  return dir.includes(WORKTREE_MARKER)
}

// Returns a denial reason, or undefined when the edit is allowed.
function guardEdit(dir: string, abs: string): string | undefined {
  const idx = dir.indexOf(WORKTREE_MARKER)
  if (idx !== -1) {
    const repoRoot = dir.slice(0, idx)
    const wt = dir.slice(idx + WORKTREE_MARKER.length).split("/")[0]
    const wtRoot = repoRoot + WORKTREE_MARKER + wt
    if (abs === repoRoot || abs.startsWith(repoRoot + "/")) {
      if (abs === wtRoot || abs.startsWith(wtRoot + "/")) return undefined
      return `Refusing to edit ${abs}: outside the active worktree for this session (${wtRoot}). Edit the file inside the worktree, not the main checkout or another worktree.`
    }
    return undefined
  }
  if (abs === dir || abs.startsWith(dir + "/")) {
    // The cwd may be a worktree outside .claude/worktrees/; say so.
    return `Refusing to edit ${abs}: this session's directory (${dir}) is not a worktree under the repo's .claude/worktrees/ tree, which is the only location this guard recognizes. Create one via the start-task skill, or move an existing worktree there: git worktree move <worktree-path> <repo-root>/.claude/worktrees/<name> (then re-point the session at it).`
  }
  return undefined
}

// Best-effort; skips dirty, non-main, and worktree checkouts.
function fastForwardMain(dir: string): void {
  if (inWorktree(dir)) return
  try {
    if (git(dir, "rev-parse", "--is-inside-work-tree") !== "true") return
    git(dir, "fetch", "origin", "main", "--quiet")
    if (git(dir, "rev-parse", "--abbrev-ref", "HEAD") !== "main") return
    const dirty = execFileSync("git", ["-C", dir, "status", "--porcelain"], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    })
    if (dirty.trim()) return
    git(dir, "merge", "--ff-only", "origin/main", "--quiet")
  } catch {
    // best-effort
  }
}

// Returns the block reason for unshipped work, else undefined (including
// "can't tell").
function unshippedWorkReason(dir: string): string | undefined {
  if (!inWorktree(dir)) return undefined
  const script = join(dir, ".claude/hooks/stop-check-unshipped-work.sh")
  try {
    const out = execFileSync("bash", [script], {
      encoding: "utf8",
      input: JSON.stringify({ cwd: dir, stop_hook_active: false }),
      stdio: ["pipe", "pipe", "ignore"],
    })
    const trimmed = out.trim()
    if (!trimmed) return undefined
    const decision = JSON.parse(trimmed) as { decision?: string; reason?: string }
    if (decision.decision === "block" && decision.reason) return decision.reason
    return undefined
  } catch {
    return undefined
  }
}

// Returns scripts/lint_docs.sh's failure output, else undefined.
function lintDocsFailure(dir: string): string | undefined {
  try {
    execFileSync(join(dir, "scripts/lint_docs.sh"), [], {
      cwd: dir,
      stdio: ["ignore", "pipe", "ignore"],
    })
    return undefined
  } catch (err) {
    const out = String((err as { stdout?: unknown }).stdout ?? "").trim()
    return out || undefined
  }
}

export default {
  id: "repo-guard",
  async setup(ctx: PluginContext) {
    fastForwardMain(ctx.location.directory)

    // 1. Worktree-scope edit guard.
    await ctx.permission.hook("evaluate", async (event) => {
      if (event.action !== "edit") return
      let dir: string
      try {
        const session = await ctx.session.get({ sessionID: event.sessionID })
        dir = session.location.directory
      } catch {
        return
      }
      for (const resource of event.resources) {
        const abs = resource.startsWith("/") ? resource : join(dir, resource)
        const reason = guardEdit(dir, abs)
        if (reason) {
          event.effect = "deny"
          event.message = reason
          return
        }
      }
    })

    // Last lint/docs output per session, so an unchanged failure isn't re-nudged.
    const lastLintDocs = new Map<string, string>()

    // 3. Unshipped-work check, armed per prompt and fired on idle. The
    //    per-commit state file makes repeats and re-entry idempotent.
    await ctx.session.hook("prompt", (event) => {
      const sessionID = event.sessionID
      void (async () => {
        try {
          await ctx.session.wait({ sessionID })
          const session = await ctx.session.get({ sessionID })
          const dir = session.location.directory
          const lint = lintDocsFailure(dir)
          if (lint && lint !== lastLintDocs.get(sessionID)) {
            await ctx.session.synthetic({
              sessionID,
              text: `repo-guard: make lint/docs fails — trim per docs/convention-concise-docs-and-comments.md:\n${lint}`,
            })
          }
          lastLintDocs.set(sessionID, lint ?? "")
          const reason = unshippedWorkReason(dir)
          if (reason) {
            await ctx.session.synthetic({
              sessionID,
              text: `repo-guard (port of Claude Code's Stop hook): ${reason}`,
            })
          }
        } catch {
          // Never block on a missing session or script.
        }
      })()
    })
  },
}
