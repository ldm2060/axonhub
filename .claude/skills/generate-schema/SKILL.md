---
name: generate-schema
description: Run Ent + gqlgen code generation after schema changes
user-invocable: true
disable-model-invocation: true
---

Run code generation for: $ARGUMENTS

1. Run `make generate` to regenerate Ent ORM and GraphQL resolver code.
2. Run `make generate-openapi` if OpenAPI schema needs updating.
3. Report any generation errors.

## Current State
- Changed schemas: !`git diff --name-only -- 'internal/ent/schema/' 'internal/server/gql/'`
