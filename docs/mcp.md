# MCP server (IMP-52)

Streamable HTTP at `/mcp` (stateless), served by the same binary as the API. The server never calls a
model: the AI client reads photos and pages and hands over structured data.

## Connecting (step 1: personal tokens)

Mint a token for a member. It is printed once; only its hash is stored.

```bash
DATABASE_URL=... go run ./cmd/token -member "Mum" -name "Claude Code - laptop"
```

The name is what the activity feed shows (`mcp:Claude Code - laptop`). Then add the server:

```bash
claude mcp add --transport http impressipes https://impressipes.bancroft.io/mcp --header "Authorization: Bearer imp_..."
```

OAuth 2.1 with dynamic client registration, needed for claude.ai and the phone app, is step 2.

## Tools

| Tool | Does |
| --- | --- |
| `list_lists`, `get_list` | Lists (by name or id), categories, items |
| `add_items` | Add many; category by name; duplicates merge their quantity |
| `check_items`, `uncheck_items`, `remove_items` | By item name or id |
| `add_recipe_to_list` | Same service as the app: scaled, merged |
| `search_recipes`, `get_recipe`, `list_tags` | Read recipes |
| `create_recipe`, `update_recipe`, `delete_recipe` | Structured ingredients and steps (with timers) |
| `attach_recipe_image` | Base64 bytes only |
| `find_similar_recipes` | Look-alikes of one or more recipes (ingredient overlap + tags) |
| `ingredient_pairings` | Recipes using an ingredient and what appears beside it |

Prompts: `import-from-photo`, `plan-week-of-meals`.

The advice tools count over the family's own recipes (`internal/pairing`). General culinary knowledge
is the client's.

## Audit and undo

Every mutation is written to `activity` with `via = mcp:<token name>`. Each tool call gets its own batch id,
returned as `batchId`, so one call's changes show as a group and undo together
(`POST /activity/batches/{batchId}/undo`). Deletes are soft. Undo answers 409 when a later change touched
the same record.
