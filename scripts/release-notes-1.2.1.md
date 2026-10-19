# Phone Connect 1.2.1

- One fixed Cloudflare tunnel automatically serves the phone upload endpoint and read-only MCP endpoint when private configuration is bind-mounted.
- Download `start.sh` and `phone-connect-container-amd64.tar.gz` together, restore your private `phone-connect-data` folder, then run `bash start.sh`. Docker and Podman are supported by the launcher.
- Local administration stays on loopback; custom local web ports are supported.
- The legacy reader download uses the configured fixed MCP base URL.
- MCP diagnostics report HTTP status and failure stage without logging tokens or message bodies.

Validated: Go tests and vet; Podman image build; clean-container startup; synthetic upload; restart persistence and unchanged upload key. Docker launcher support is implemented but not tested with a Docker engine on this PC.

Cloudflare domain activation and OAuth registration are account-specific first-time setup. Credentials and personal records are not included in this public release. Restoring the full bind-mounted data folder preserves your configuration and history. Do not run two independent servers with the same tunnel credentials simultaneously.
