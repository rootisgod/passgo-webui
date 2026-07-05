# Reusable LLM Settings Kit

PassGo Web now has enough repeated LLM provider setup code to justify extracting a small reusable kit for other apps. The target is the whole settings experience: provider presets, server-side key handling, model discovery, native CLI login detection, and a reusable Vue dialog.

This should be a kit, not a provider framework. Product-specific agent behavior, tools, prompts, confirmation flows, and chat UX stay in each app.

## Reusable Boundary

The reusable backend boundary should own:

- Provider IDs and labels: Vercel AI Gateway, OpenAI-compatible, local, Codex CLI, Claude Code.
- Default provider config: gateway base URL and default model.
- Secret handling: stored-key vs server-env vs no-key state, without exposing secrets to the browser.
- Base URL validation: HTTPS remote providers plus loopback HTTP for local providers.
- Model discovery: OpenAI-compatible `/models` fetch, gateway filtering/sorting/limits, static native CLI model choices.
- Native auth status: fixed `codex login status` and `claude auth status` checks with no shell and discarded output.
- Native CLI model calls, if wanted by the app: fixed command args, structured JSON response parsing, and clear errors.

The reusable frontend boundary should own:

- Provider preset buttons.
- Native login detection rows and `Use` selection.
- Base URL and API key fields for non-native providers.
- API key source hints such as `server env` or `configured`.
- Model picker fed by a `GET /models` style endpoint.
- Read-only toggle as a generic boolean setting.

The host app should provide:

- The API client functions used by the component.
- Persistence for `LLMConfig`.
- The chat request/stream implementation.
- Product-specific tools and confirmation policy.
- Product-specific copy or labels if needed.

## API Contract

A reusable dialog can work against this minimal API shape:

```http
GET /chat/config
PUT /chat/config
GET /chat/models
GET /chat/native-auth
```

Config response:

```json
{
  "provider": "vercel_ai_gateway",
  "base_url": "https://ai-gateway.vercel.sh/v1",
  "model": "anthropic/claude-sonnet-5",
  "has_api_key": true,
  "api_key_source": "environment",
  "read_only": false
}
```

Config update:

```json
{
  "provider": "codex_cli",
  "base_url": "",
  "api_key": "",
  "model": "codex-default",
  "read_only": true
}
```

Model list:

```json
[
  { "id": "anthropic/claude-sonnet-5", "name": "Claude Sonnet 5" }
]
```

Native auth status:

```json
{
  "providers": [
    {
      "id": "codex",
      "label": "ChatGPT (Codex CLI)",
      "command": "codex login status",
      "installed": true,
      "authenticated": true,
      "status": "authenticated"
    }
  ]
}
```

## Suggested Package Shape

For Go apps:

```text
pkg/llmsettings/
  config.go          provider IDs, defaults, config structs
  resolve.go         key source resolution and env fallback
  urls.go            base URL validation and host comparison
  models.go          /models fetch and normalization
  native_auth.go     CLI login detection
  native_cli.go      optional structured Codex/Claude adapters
```

For Vue apps:

```text
packages/llm-settings-vue/
  LLMSettingsModal.vue
  useLLMSettings.js
  providerPresets.js
  apiContract.md
```

The Vue component should accept injected functions rather than importing an app-specific API client:

```js
{
  getConfig,
  saveConfig,
  listModels,
  getNativeAuthStatus
}
```

That keeps it copyable across apps with different auth, routing, and toast systems.

## Extraction Plan

1. Keep PassGo's current implementation as the reference implementation.
2. Move `internal/aiaccess` to `pkg/llmsettings` once one more app needs it.
3. Convert `ChatSettingsModal.vue` into a prop-driven component that receives API functions and optional toast callbacks.
4. Leave PassGo's agent loop in `internal/api`; only the provider setup and optional native structured model call should be reusable.
5. Add a small example app or fixture that proves the component works with the API contract above.

The current code intentionally stops one step short of this extraction. That keeps the PassGo feature shippable while preserving a clear path to a reusable package.
