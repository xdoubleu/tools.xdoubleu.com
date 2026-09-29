# ADR-0026: recipes gets one draft-only MCP write tool

- Status: Accepted
- Issues: #1989 (part of epic #385)
- Affects: `api/apps/recipes/mcp.go`, `api/apps/recipes/mcp_create_recipe.go`, `api/internal/mcptools/`

## Context

Recipes are imported in bulk from PDFs, DOCX files and recipe sites. An agent
can read those sources but read-only tools leave it no way to save the result,
and a server-side importer would need its own LLM. ADR-0023 requires a separate
justification and ADR for each app's mutating tools.

## Decision

- `recipes_create_recipe` wraps the `CreateRecipe` Connect handler and always
  sets `is_draft = true`; it has no `is_draft` argument. The user reviews and
  publishes the draft in the app.
- It uses the same trust model as ADR-0023: `RequireAppAccess(ctx, "recipes")`,
  and the user and family come from context, never from an argument.
- There are no update or delete tools, so an agent cannot change a recipe the
  user has already reviewed.
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
- `TestMCPTools_CreateRecipeIsDraft` (`mcp_internal_test.go`) proves the write
  goes through the service layer and lands as a draft.

## Revisit when

A client needs to edit recipes after import, or the drafts that agents create
need too much manual cleanup.
