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
- **One model, `TASTE_GENERATOR_MODEL`** (default `mistralai/mistral-large-4-0`).
  It writes the profile, proposes candidates and gives each a 0-100 score in
  the same call. Catalog verification, not a second model, is the check on
  its picks.
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

- **Direct provider APIs**: one key and client per vendor.
- **A separate judge model** scoring the candidates: an independent second
  opinion, but a second env var, call and failure path for one user.
- **Generating inside the request**: exceeds kamal-proxy's response timeout.
- **A monthly token budget enforced in the app**: more code than the key's
  credit limit for the same protection.

## Consequences

- Profile and recommendation quality depends on the configured model. A
  model ID OpenRouter retires shows up as the profile's last error.
- A model scoring its own picks may rate them generously, so scores rank
  within a run rather than measure absolute fit.
- A user's consumption digest (titles, ratings, tags) is sent to OpenRouter
  and the model's provider.

## Revisit when

Costs approach the key's credit limit, or a stage needs a model OpenRouter
doesn't serve.
