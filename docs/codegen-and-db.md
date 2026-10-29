# API contract and database

## OpenAPI codegen (IMP-24)

`libs/api-spec/openapi.yaml` is the source of truth. `mise run codegen` regenerates:

| Client | Tool | Output (committed) |
| --- | --- | --- |
| Go | oapi-codegen (strict server) | `apps/api/internal/api/api.gen.go` |
| TypeScript | openapi-typescript + openapi-fetch | `apps/web/src/api-gen/schema.gen.ts` |
| Swift | swift-openapi-generator | `libs/api-spec/swift/Sources/ApiClient/Generated` |

CI regenerates all three and fails if the committed output differs (`libs/api-spec/scripts/check`).
The Go handlers implement `api.StrictServerInterface`, so changing the spec without updating
`apps/api/internal/server` is a compile error.

## Database (IMP-25)

- Migrations: goose SQL files in `apps/api/internal/db/migrations`, embedded in the binary and applied on boot when `DATABASE_URL` is set.
- Queries: `apps/api/internal/db/queries/*.sql`; `mise run sqlc` (in `apps/api`) regenerates `internal/db/dbgen`.
- Tests: `internal/db` uses Testcontainers (Postgres 17) and needs Docker; they skip under `go test -short` or without Docker.
- The REST handlers still use the in-memory `family` store; moving them onto sqlc is follow-up work.
