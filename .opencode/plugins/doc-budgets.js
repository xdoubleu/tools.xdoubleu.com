// OpenCode port of the Claude Code PostToolUse hook. Copy to
// .opencode/plugins/doc-budgets.js and set DOC_BUDGETS_DIR to a checkout of
// plugins/doc-budgets (or vendor its scripts/ directory next to it).
import { spawnSync } from "node:child_process"
import path from "node:path"

const scripts = path.join(process.env.DOC_BUDGETS_DIR ?? ".", "scripts")

export const DocBudgets = async ({ directory }) => ({
  "tool.execute.after": async (input, output) => {
    if (!["edit", "write"].includes(input.tool)) return
    const file = output?.args?.filePath ?? input.args?.filePath
    if (!file) return
    const errors = ["lint_docs.sh", "lint_doc_paths.sh"]
      .map((s) => spawnSync("bash", [path.join(scripts, s), file], { cwd: directory, encoding: "utf8" }))
      .filter((r) => r.status !== 0)
      .map((r) => r.stderr.trim())
    if (errors.length) throw new Error(errors.join("\n"))
  },
})
