# pilot-001 schema-mismatch golden workspace

This fixture is intentionally malformed helper state used by the black-box
pilot-001 E2E harness. It is copied into a temporary git repository and checked
through the public `schema validate` CLI surface.

The fixture includes config/events files so it remains shaped like a real `.kkachi` workspace, while the test intentionally targets `status.json` schema validation.
