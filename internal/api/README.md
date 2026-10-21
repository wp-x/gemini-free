# Compatible API

`New(Options)` exposes `/health`, `/v1/models`, `/v1/chat/completions`,
`/v1/messages`, and `/v1/messages/count_tokens` through a standard `http.Handler`.
`Generator` receives the request context, including cancellation.

Text streams emit upstream increments. Tool streams wait until the complete
structured output passes tool name, arguments object, required choice, and
parallel call checks. A malformed upstream tool response returns 502.
Unsupported input content blocks and invalid tool selections return 400.

The Gemini web request does not carry these controls:

- `max_tokens`, `max_completion_tokens`, `temperature`, `top_p`, `top_k`
- `stop`, `stop_sequences`, `frequency_penalty`, `presence_penalty`, `logit_bias`
- `logprobs`, `top_logprobs`, `seed`, `response_format`, `n`
- `thinking`, `output_config`, `service_tier`, `stream_options`
- `prediction`, `modalities`, `audio`, `verbosity`

The API accepts these fields for existing client compatibility and lists each
present field in `X-Gemini-Unsupported-Parameters`. Their requested behavior is
not applied. `reasoning_effort` is validated and forwarded only for Argon routes.

Token usage and token counting are local UTF-8 byte estimates, not upstream
tokenizer measurements. Responses include `estimated: true` and the
`X-Gemini-Token-Usage` header.

Nonstreaming and buffered tool responses include `X-Gemini-Actual-Mode` and
`X-Gemini-Thinking-Level`. Response bodies and final text SSE events carry the
same values in a `gemini` object. Local model names describe requested routes;
the actual mode identifies the upstream response.

OpenAI stream failures are `data: {"error": ...}` followed by `[DONE]`.
Anthropic stream failures are `event: error` with `type: "error"`. Neither emits
a successful stop event after a generation error.
