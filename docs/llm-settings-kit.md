# Reusable LLM Settings Kit

The reusable LLM provider settings work has been extracted to:

https://github.com/iaingblack/llm-kit

That repo contains:

- A Go package for provider config, Vercel AI Gateway defaults, server-side key handling, model discovery, native Codex/Claude CLI login detection, and optional native structured CLI calls.
- A Vue package with a headless `useLLMSettings()` composable and a default restylable `LLMSettingsModal.vue`.
- A README with the API contract, clone/copy workflow, and integration examples.

PassGo Web currently keeps a local implementation of the same concepts in `internal/aiaccess`, `internal/api`, and `frontend/src/components/chat/ChatSettingsModal.vue`. A future cleanup can replace those local pieces with `github.com/iaingblack/llm-kit/llmkit` and the Vue source from `llm-kit/vue`.

PassGo-specific code should stay in this repository:

- Multipass tool definitions and execution.
- The chat agent loop and SSE behavior.
- Read-only mode semantics for VM tools.
- Destructive-tool confirmation policy.
- PassGo config persistence, import, and export behavior.

The reusable kit should remain limited to provider setup and access plumbing.
