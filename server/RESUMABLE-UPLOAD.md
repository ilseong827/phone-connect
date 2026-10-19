# Resumable recording uploads

Server v1.2.4 and Android 0.5.2 send recordings larger than 32 MiB in separate
8 MiB HTTPS requests. Smaller recordings keep the existing multipart endpoint.
This works through Cloudflare's 100 MB request limit without changing the plan.
Upgrade the server before installing the new APK; older servers do not implement
the new endpoints. The app retains unsuccessful uploads for retry.

All endpoints require the normal upload Bearer key and a 64-character hexadecimal
`Idempotency-Key`. Read-only MCP credentials cannot upload.

1. `POST /v1/recording-upload/start`: recording metadata JSON (maximum 64 KiB).
   Returns `id`, `chunkSize`, and the first missing `nextChunk`. An already saved
   recording returns `accepted: true`. Recording size must be 1 byte–1 GiB.
2. `POST /v1/recording-upload/chunk?index=N`: raw bytes with `X-Chunk-SHA256`.
   Each full chunk is exactly 8 MiB; the final one contains the remainder.
   Wrong size/checksum is rejected. Identical retries are safe; conflicting
   retries return 409. Metadata for an existing upload cannot change.
3. `POST /v1/recording-upload/complete`: assembles all chunks and uses the normal
   recording storage/transcription pipeline. Missing chunks return 409.
   Duplicate completion requests return the original acknowledgement.

Chunks and metadata are stored under `/data/recording-uploads/<id>/`, so keep
the existing `/data` bind mount when recreating the container. Incomplete uploads
are not visible as recordings. Successful completion removes the staging folder.
Abandoned uploads remain resumable on disk; they have no automatic expiry yet.
During assembly, allow disk space for both the chunks and the assembled original.

The app verifies source size/modification time before and after transfer. It
resumes at the server's first missing chunk after a network error or process
restart, and removes its queue entry only after the final accepted response.
Android background scheduling still controls when retries run; opening the app
and syncing can trigger a retry sooner.
