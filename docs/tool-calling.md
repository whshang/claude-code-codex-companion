# Tool Calling Enhancement (Zero‑Config)

This project provides a zero‑config “Toolify‑style” tool calling enhancement layer to enable function calls for models and providers that don’t natively support it.

What it does (today)
- Auto‑injects a strict system prompt when the client request includes `tools`.
- Parses tool calls from the assistant text using a unique trigger signal + XML wrapper.
- Returns a standard tool call response in the client’s native format:
  - OpenAI: `choices[0].message.tool_calls` + `finish_reason: "tool_calls"`
  - Anthropic: content blocks with `type: "tool_use"`
- Only non‑streaming path is integrated now. Streaming (SSE) integration is planned.

What it does not do (yet)
- SSE streaming tool calls (delta mapping to OpenAI/Anthropic). The detector and APIs exist and will be wired up in Phase 2.

No global configuration
- The old global `tool_calling` config is removed. We use endpoint‑level learning + lightweight toggles instead.

Endpoint‑level controls
- `native_tool_support` (optional): whether the endpoint natively supports tool calls. If omitted, the system will learn it via tests.
- `tool_enhancement_mode` (optional): `auto | force | disable`
  - `auto` (default): if native support is `true`, do not inject; otherwise inject.
  - `force`: always inject the enhancement.
  - `disable`: never inject the enhancement.

Admin UI updates
- Endpoint modal (Advanced) now exposes the above two fields (non‑required).
- Endpoints list shows compact config chips: Tool/native or mode, OpenAI preference (chat/resp/auto), Model‑rewrite on/off.

Automatic learning and safety fallback
- Admin “Test Endpoint(s)” learns native tool support per endpoint and writes it back.
- During runtime, if a request includes `tools` and the upstream returns a business error, the proxy learns and persists `native_tool_support=false` and sets `tool_enhancement_mode=force` to prevent repeated failures from “claimed but broken” tool support.

CLI probe (report‑only)
- `go run ./cmd/probe-endpoints -config config.yaml | tee tmp/endpoint_probe_all.json`
- Tests every endpoint (ignoring enabled state) and reports Anthropic/OpenAI basic availability and native tool support.

Timeouts and avoiding “context deadline exceeded”
- The Admin test timeouts are relaxed to 20s to reduce false negatives on slow providers.
- The proxy path does not impose an overall request deadline to preserve streaming.

Planned: streaming/SSE
- Phase 2 will wire `StreamingDetector` to detect the trigger + `</function_calls>` in the stream and map them to the client’s delta format.

