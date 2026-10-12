# ADR-0032: the taste app's LLM stages run on OpenRouter, from jobs

- Status: Accepted
- Issues: #2423, #2424 (epic #2233)
- Affects: `api/internal/openrouter/`, `api/apps/taste/`, `OPENROUTER_API_KEY`

## Context

The taste app writes a readable profile from the consumption log and, next,
recommends content across domains. With one user, collaborative filtering is
out, so both stages need an LLM with broad world knowledge. Generation takes
tens of seconds, which is longer than a request may run (ADR-0017).

## Decision

- **OpenRouter as the only LLM provider**, reached through a minimal
  chat-completions client (`internal/openrouter`). It asks for strict JSON
  schema answers and logs token usage and cost on every call.
- **One model per stage, set by config:**
  - `TASTE_GENERATOR_MODEL` (default `anthropic/claude-sonnet-5`) writes the
    profile and generates candidates.
  - `TASTE_JUDGE_MODEL` (default `google/gemini-3.6-flash`), a cheaper model
    from another family, only scores candidates.
- **The generator may search the web** for candidates through OpenRouter's
  `openrouter:web_search` server tool. Each generation caps it with
  `max_results`, `max_total_results` and `max_tool_calls`. The profile itself
  is written from the log alone.
- **Generation only runs in jobs:**
  - An hourly job writes profiles that are pending or were last attempted
    over 7 days ago.
  - `RefreshProfile` marks a profile pending and force-runs the job. It is
    refused within 6 hours of the last request.
  - A failed attempt counts as an attempt, so a broken model isn't retried
    every hour.
- **Cost guard:** the weekly cadence and the 6-hour refresh limit. The hard
  ceiling is the credit limit on a dedicated `OPENROUTER_API_KEY`, separate
  from the agent routines' key. There is no dollar tracking in the app.
- **Without a key** the stages are off: the job idles and the UI says so.

## Alternatives considered

- **Direct provider APIs**: one key and client per vendor, and the
  cross-family judge would double that.
- **Generating inside the request**: exceeds kamal-proxy's response timeout.
- **A monthly token budget enforced in the app**: more code than the key's
  credit limit for the same protection.

## Consequences

- Profile and recommendation quality depends on the configured models. A
  model ID OpenRouter retires shows up as the profile's last error.
- A user's consumption digest (titles, ratings, tags) is sent to OpenRouter
  and the model's provider.

## Revisit when

Costs approach the key's credit limit, or a stage needs a model OpenRouter
doesn't serve.
