# ADR-0062: A chat question is a `factory_chat_ask` call answered from a card

- Status: Accepted
- Issues: T-803, T-799
- Affects: `api/internal/platform/factory` (`mcp`, `services/chat_asks.go`), `proto/factory/v1/factory.proto` (`AnswerChatRequest.ask_id`), `factory-runner/internal/runner/chat.go`, `web/components/factory/ChatAskCard.tsx`

## Context

A chat agent needs to put a choice or confirmation to the person. ACP only standardises permission requests, which wardcase auto-allows; whether OpenCode surfaces its own question tool over ACP is unverified, and the sandbox that built this could not check.

## Decision

- **Channel:** the board's MCP endpoint gets `factory_chat_ask` (question, 2 to 4 options, `multi_select`). Only a chat job's own account may call it. Tool names must start with the app, hence the prefix.
- **Persisted:** a call appends an `ask` event (its seq is the question's id) and moves the session to `asking` in one statement, so the card survives a reload.
- **Answer:** `AnswerChat` with `ask_id` appends an `ask_answer` event and sets the session `working`. It is not a `user_answer`, which the bridge would send to the harness as a new prompt. The waiting call returns the text as the tool result. The card offers the options and free-text "other".
- **Deadline:** a call waits 15 seconds (MCP requests have a 25-second budget, [adr-0017](adr-0017-long-request-handler-deadlines.md)), then returns `waiting` with the `ask_id`; the agent calls again with it until answered. A closed chat fails the call.
- **Prompt:** the runner sends instructions naming the tool ahead of the first message, since ACP has no system prompt.

## Alternatives considered

- Map an OpenCode question over ACP: unverified, and it would tie the card to one harness.
- Block a call until answered: breaks the request deadline.

## Consequences

- The agent may still use its own question tool, which shows as prose.
- Answering polls the events table every 500 ms while a call waits.

## Revisit when

OpenCode's question reaches the runner over ACP, or MCP calls may outlast the request budget.
