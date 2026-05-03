# pilot-001 invalid-events-jsonl golden workspace

This fixture has an invalid JSONL event line. The E2E harness expects
`project doctor --json` to fail closed instead of treating malformed history as
recoverable.
