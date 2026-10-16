Phone Connect v1.2.0 — Podman and OAuth MCP preview

- Single Podman container with bind-mounted data, web UI, FFmpeg and local transcription.
- Android v0.5.0 re-syncs recoverable records after changing servers.
- Remote MCP with eight read-only tools, OAuth discovery and RS256 token validation restricted to the owner.
- Local MCP settings page; optional fixed Cloudflare tunnel token file.
- Go tests, SDK HTTP integration, HTTPS discovery/JWKS and authentication rejection cases passed.

MCP is disabled until an OAuth provider and public address are configured. Actual ChatGPT account linking, Dot/mobile queries and provider refresh flow remain untested. See server/MCP.md. This prerelease does not claim completed ChatGPT integration.

The APK uses the existing project debug signing key. Container archive is for Linux amd64. It may be added after the APK/native assets while the image build finishes.
