# LiFi MCP Server

An MCP server integrating with the [LI.FI API](https://li.quest) for cross-chain swap functionality.

## Guidelines

See [AGENTS.md](./AGENTS.md) for build commands and code style guidelines.

## Project Context

- **Language**: Go 1.23.6+ with mcp-go v0.39.1+
- **API**: LI.FI REST API at li.quest
- **Protocol**: Model Context Protocol (MCP) via stdio or in-process transport
- **Keystore**: Transaction tools require `--keystore` flag with Ethereum keystore

## Security

⚠️ **Use test wallets only.** Never use main wallets or wallets with significant funds.
