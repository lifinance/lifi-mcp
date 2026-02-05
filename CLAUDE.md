# LiFi MCP Server

An MCP server integrating with the [LI.FI API](https://li.quest) for cross-chain swap functionality.

## Guidelines

See [AGENTS.md](./AGENTS.md) for build commands and code style guidelines.

## Project Context

- **Language**: Go 1.24.0+ with mcp-go v0.39.1+
- **API**: LI.FI REST API at li.quest
- **Protocol**: Model Context Protocol (MCP) via Streamable HTTP transport on `/mcp` endpoint
- **Multi-tenant**: API keys passed per-request via `Authorization: Bearer` or `X-LiFi-Api-Key` header

## Security

All tools are read-only — no transaction signing or wallet management. API keys are passed per-request and never stored in server state.
