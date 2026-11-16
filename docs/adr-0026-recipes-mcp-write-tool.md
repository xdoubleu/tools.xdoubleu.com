# ADR-0026: recipes gets draft-preserving MCP write tools

- Status: Accepted
- Issues: #1989 (part of epic #385)
- Affects: `api/apps/recipes/mcp.go`, `api/apps/recipes/mcp_create_recipe.go`, `api/apps/recipes/mcp_update_recipe.go`, `api/internal/platform/mcptools/`

## Context

Recipes are imported in bulk from PDFs, DOCX files and recipe sites. An agent
can read those sources but read-only tools leave it no way to save the result,
and a server-side importer would need its own LLM. ADR-0023 requires a separate
justification and ADR for each app's mutating tools.

## Decision

- `recipes_create_recipe` wraps the `CreateRecipe` Connect handler and always
  sets `is_draft = true`; it has no `is_draft` argument. The user reviews and
  publishes the draft in the app.
- `recipes_update_recipe` wraps the `UpdateRecipe` Connect handler. It has no
  `is_draft` argument either: it reads the recipe's current draft state and
  preserves it, so an agent can edit a recipe (e.g. set categories or group
  ingredient alternatives) but never publish or unpublish it. Unlike create,
  an update applies to the recipe as it stands, including one already
  published.
- Both use the same trust model as ADR-0023: `RequireAppAccess(ctx, "recipes")`,
  and the user and family come from context, never from an argument.
- The import conventions (language, units, step format, duplicate check) live
  in the tool description, so every MCP client gets them.
- The second caller moves `addWriteTool` into `mcptools.AddWriteTool`, as
  ADR-0023's "Revisit when" asks.

## Alternatives considered

- **In-app importer with server-side LLM parsing**: needs an API key and an
  import UI, while the agent already has the source open.
- **Caller-controlled `is_draft`**: an agent could then publish unreviewed
  recipes to the whole family.
- **A repo skill for the conventions**: claude.ai web and mobile clients
  can't load it.

## Consequences

- A `recipes` app-access grant lets an agent create draft recipes in the
  caller's family.
- The duplicate check depends on the agent calling `recipes_list_recipes`
  first; nothing in the server enforces it.
- `TestMCPTools_CreateRecipeIsDraft` and `TestMCPTools_UpdateRecipe`
  (`mcp_internal_test.go`) prove the writes go through the service layer and
  land as (respectively) a draft and an update preserving the draft flag.

## Revisit when

The drafts that agents create need too much manual cleanup, or updates need
to change more than an existing recipe's content (e.g. a delete tool).
