// repo-guard — OpenCode port of the task-lifecycle enforcement hooks that
// Claude Code runs from .claude/settings.json (issue #1817). Three guards,
// mirroring their Claude counterparts' semantics exactly:
//
// 1. Worktree-scope edit guard (Claude: PreToolUse on Edit/Write/NotebookEdit).
//    A session inside .claude/worktrees/<wt> may only edit files under that
//    worktree; a session in the main checkout may not edit repo files at all
//    until start-task creates a worktree. Enforced via permission.hook, which
//    runs after configured rules and can flip allow → deny.
//
// 2. Session-start main fast-forward (Claude: inline SessionStart hook).
//    Plugin setup runs when OpenCode loads this location, which is the
//    service-start analogue of Claude's per-session hook: fetch origin/main
//    and ff-merge it when the main checkout is on main and clean.
//
// 3. Unshipped-work check (Claude: Stop hook). Approximated — OpenCode has no
//    blocking stop, so this arms a waiter on every admitted prompt that fires
//    when the session next goes idle, and nudges the session with a synthetic
//    message instead of blocking. The check itself reuses the Claude script
//    (.claude/hooks/stop-check-unshipped-work.sh) verbatim, including its
//    per-commit state file under the git common dir, so the two harnesses
//    share dedupe state: a commit already flagged by one is not re-flagged
//    by the other.
//
// The grep→ast-grep reminder hook is deliberately not ported: AGENTS.md
// states the rule, and OpenCode hooks have no per-tool context injection.
//
// No `@opencode/plugin` import: the loader resolves bare imports relative to
// this directory, which has no node_modules, and `Plugin.define` is an
// identity helper — a plain `{ id, setup }` default export is the same
// contract. The structural types below mirror the documented Context
// (opencode.ai/v2/docs/build/plugins); keep them in sync if the API moves.
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

// Port of the PreToolUse worktree-scope guard in .claude/settings.json.
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
    // The cwd may still be a legitimate git worktree — the guard only
    // recognizes worktrees under the repo's .claude/worktrees/ tree, so say
    // that (and the escape hatch) instead of mislabeling it "main checkout".
    return `Refusing to edit ${abs}: this session's directory (${dir}) is not a worktree under the repo's .claude/worktrees/ tree, which is the only location this guard recognizes. Create one via the start-task skill, or move an existing worktree there: git worktree move <worktree-path> <repo-root>/.claude/worktrees/<name> (then re-point the session at it).`
  }
  return undefined
}

// Port of the inline SessionStart ff-merge hook in .claude/settings.json:
// best-effort, silent on every failure, never touches a dirty or non-main
// checkout, and skipped inside worktrees (those are task branches by design).
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
    // Best-effort backstop, same as the Claude hook: any failure is a no-op.
  }
}

// Runs the shared Stop-hook script against a session directory. Returns the
// block reason when the branch has finished-but-unshipped work, else
// undefined ("can't tell" also means don't fire, per the script's own rule).
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

    // 3. Unshipped-work check. Armed on every admitted prompt; the waiter
    //    resolves when that session next goes idle. The script's per-commit
    //    state file makes repeat firings (multiple waiters, multiple idles)
    //    idempotent, and the synthetic message it may trigger re-enters the
    //    prompt hook without looping: same commit → already flagged → silent.
    await ctx.session.hook("prompt", (event) => {
      const sessionID = event.sessionID
      void (async () => {
        try {
          await ctx.session.wait({ sessionID })
          const session = await ctx.session.get({ sessionID })
          const reason = unshippedWorkReason(session.location.directory)
          if (reason) {
            await ctx.session.synthetic({
              sessionID,
              text: `repo-guard (port of Claude Code's Stop hook): ${reason}`,
            })
          }
        } catch {
          // Session removed, interrupted, or script missing — never block on it.
        }
      })()
    })
  },
}
