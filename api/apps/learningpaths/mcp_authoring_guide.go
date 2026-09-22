package learningpaths

// Shared authoring-guidance strings for the learningpaths MCP server.
//
// These live with the code (not in a repo markdown skill that must be
// re-uploaded elsewhere) so every MCP client — a ChatGPT-style connector
// included — sees the current authoring guidance at connect time, from the
// tool descriptions and the learningpaths://authoring-guide resource, with no
// manual sync. Keep the data model and arg shape here in lockstep with the
// mcp*Arg structs and toProto* helpers in mcp.go; that is the source of
// truth for what these describe.

const (
	// authoringGuideURI is the resource URI for the full learning-paths
	// authoring guidebook. Resource read handlers must be registered in
	// RegisterMCPTools alongside the tools.
	authoringGuideURI  = "learningpaths://authoring-guide"
	authoringGuideMIME = "text/markdown"
)

// mcpCreatePathDescription is the description for the learningpaths_create_path
// tool. It carries the data model and the confirm-before-create rule directly
// on the tool so a model sees them without a separate resource fetch.
const mcpCreatePathDescription = "Creates a new learning path: a title, a goal " +
	"(what the learner will be able to do afterward), a freeform routine (the " +
	"recurring cadence, e.g. \"30 min every weekday morning\"), ordered modules " +
	"each with ordered items, and a freeform resources list. Modules, items, and " +
	"resources are created in the order given and keep that order. Always " +
	"confirm the full proposed tree with the user before calling this — it " +
	"persists real data. Mutating — see this app's ADR for why."

// mcpUpdatePathDescription is the description for learningpaths_update_path.
const mcpUpdatePathDescription = "Wholesale-replaces a learning path's " +
	"title/goal/routine/modules/resources — the full tree, not a partial patch: " +
	"every field you pass replaces what's there, and modules/resources replace " +
	"the prior lists entirely. Read the current path first with " +
	"learningpaths_get_path and pass the complete desired tree. Mutating — see " +
	"this app's ADR for why."

// mcpRecordProgressDescription is the description for
// learningpaths_record_progress.
const mcpRecordProgressDescription = "Marks a single item complete or " +
	"incomplete by its item_id (read from learningpaths_get_path after " +
	"creating) without resending the whole tree. Mutating — see this app's " +
	"ADR for why."

// learningPathsAuthoringGuide is the full guidebook served at
// learningpaths://authoring-guide. It is the whole-curriculum resource for MCP
// clients that read resources (Claude Code/OpenCode); the tool descriptions
// above carry the same essentials for clients (like ChatGPT) that only look at
// tool metadata.
const learningPathsAuthoringGuide = `# Authoring learning paths on tools.xdoubleu.com

A learning path is a curriculum a user owns: a title, a goal (what they want
to be able to do afterward), and a freeform routine (how often they'll work at
it, e.g. "30 min every weekday morning"), plus two ordered lists:

- Modules — the stages, each with a title and ordered items.
- Items — the concrete, checkable steps in a module. Each has a freeform type
  (e.g. read, do, checkpoint, watch, practice) and a description saying exactly
  what to do.
- Resources — freeform supporting entries (a URL, "Book: …"), not steps to
  check off.

Everything operates only on the calling user's own paths (scoped server-side).

## Tools

- learningpaths_list_paths — all of the user's paths (read).
- learningpaths_get_path — one path's full tree; returns the ids you need for
  updates and progress (read).
- learningpaths_get_progress — completion counts, overall and per module (read).
- learningpaths_create_path — create a path with its full tree (write).
- learningpaths_update_path — wholesale-replace title/goal/routine/modules/
  resources; not a partial patch (write).
- learningpaths_record_progress — toggle one item's completed flag by item_id
  (write).

## create_path / update_path argument shape

    title        string   required
    goal         string   optional  (what the learner can do afterward)
    routine      string   optional  (freeform cadence, e.g. "daily morning")
    modules[]    ordered
      title      string   required
      items[]    ordered
        type         string  optional (freeform verb: read / do / checkpoint …)
        description  string  required
        completed    bool    optional
    resources[]  ordered
      text       string   required (a URL, "Book: …", etc.)

## record_progress argument shape

    item_id    string   required (from learningpaths_get_path)
    completed  bool     required

## Authoring workflow

1. Elicit before drafting: topic/goal, current level, time budget, cadence.
   Don't guess a curriculum uninvited — a path built on a guessed goal or an
   unkeepable cadence is wasted persisted data.
2. Draft the full tree and confirm the full proposed tree with the user before
   creating anything (a wrong curriculum is far cheaper to review as text than
   to delete after it's stored). This is unconditional before any write.
3. Create with learningpaths_create_path.
4. Verify with learningpaths_get_path and report the path id and its web URL
   https://tools.xdoubleu.com/learningpaths/<id>.
5. Refine with learningpaths_update_path (resend the entire tree) or check
   items off with learningpaths_record_progress(item_id, completed).

## Curation rubric

- Goal framing: state what the learner can do afterward, not just know.
- Realistic routine: a real cadence with a time bound ("15 min every weekday
  morning"), not "take a course".
- Prerequisite ordering: modules progress foundations → advanced.
- Actionable, checkable items: not "learn X" but a concrete completing action
  ("read chapter 3 and summarize it"). Each item is checkable.
- Shape: roughly 3–6 items per module; one goal per path. Suggest a more
  focused path before writing 20+ modules or 60+ items.
- Checkpoint items: include a checkpoint-type item at a module's mid-point or
  end so progress can register.
- Resource curation: link a real books library entry or feeds item when one
  matches, else a freeform URL/description.

## Guardrails

- Never create empty modules (a module with no items).
- Always confirm before a write.
- Use record_progress for completion toggles, not update_path; use update_path
  only for structural changes.
- update_path replaces: pass the full title/goal/routine AND the full
  modules/resources arrays read from get_path, or you'll drop what you omit.
- Scoping is automatic: these tools can only touch the calling user's own
  paths.
`
