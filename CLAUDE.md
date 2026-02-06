# LiFi MCP Server

An MCP server integrating with the [LI.FI API](https://li.quest) for cross-chain swap functionality.

## Guidelines

See [AGENTS.md](./AGENTS.md) for build commands and code style guidelines.

## Project Context

- **Language**: Go 1.24.0+ with mcp-go v0.39.1+
- **API**: LI.FI REST API at li.quest
- **Protocol**: Dual transport — Streamable HTTP (`/mcp`) + Stdio
- **HTTP mode** (default): Multi-tenant, API keys per-request via `Authorization: Bearer` or `X-LiFi-Api-Key` header
- **Stdio mode**: Single-tenant, API key from `LIFI_API_KEY` env var (for local MCP clients)

## Security

All tools are read-only — no transaction signing or wallet management. In HTTP mode, API keys are passed per-request and never stored in server state. In stdio mode, the API key is read from the `LIFI_API_KEY` environment variable.
