# pilot-001 status-event-mismatch golden workspace

This fixture has syntactically valid helper status/events, but
`status.last_event_id` intentionally lags the event log tail. The E2E harness
expects `project doctor --json` to fail closed on the coherence check.
