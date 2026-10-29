---
name: impressipes
description: Work with the family's Impressipes recipe box and shared shopping lists through its MCP connector. Use when the user wants to save a recipe from a photo or page, search or edit saved recipes, plan meals for the week, or add, check off or remove items on a grocery list.
---

# Impressipes

Impressipes is the family's recipe box and shared shopping lists. This skill tells you how to use its MCP
connector well. The connector must already be added (a custom connector pointing at the site's `/mcp`
address, or a personal token). If the tools below are not available, say so and ask the user to connect it.

You do the reading and thinking; the tools only store structured data.

## Saving a recipe from a photo or page

1. Read the whole source carefully before calling anything.
2. `search_recipes` with the title first, so nothing is saved twice. If it exists, offer to update it instead.
3. `list_tags` and reuse existing tags where they fit.
4. `create_recipe` with `sourceKind: "photo"` (or the fitting kind for a page):
   - split each ingredient into `item`, `quantity` (a number), `unit` and `prepNote`
   - keep the source's section headings
   - one instruction per step; set `timerSeconds` when the step names a wait
5. Never invent what the source does not say. Put doubts in the notes.
6. Give the recipe a small thumbnail, but never use the photo you were given of the recipe card or cookbook
   page: that is a picture of text, not of the food. Instead, in this order:
   - If you can generate images, make a small, appetising picture of the finished dish (a square thumbnail
     is plenty) and `attach_recipe_image` it.
   - Otherwise, for a recipe from a web page, use the page's own picture of the dish (its `og:image`, or the
     main recipe photo) when you hold its bytes.
   - If you can do neither, skip the image. A recipe without one is fine; the app shows a placeholder.
   A picture shown in chat is not available to tools, so only attach files whose bytes you actually hold.
7. Finish by telling the user what was saved.

## Planning meals

1. `search_recipes` (favourites first) and `find_similar_recipes` to vary the week; avoid the same main
   ingredient on consecutive nights. `ingredient_pairings` helps when the user starts from an ingredient.
2. Show the plan and wait for a yes before changing anything.
3. Then `add_recipe_to_list` for each recipe. It scales and merges quantities the same way the app does.
4. Report what was added and what merged.

## Shopping lists

- `list_lists` / `get_list` to see lists, categories and items (lists resolve by name or id).
- `add_items` takes many at once, with category by name; duplicates merge their quantity.
- `check_items`, `uncheck_items` and `remove_items` take item names or ids.

## Good habits

- Ask before deleting a recipe or removing many items.
- Every change is audited as coming from this connector and can be undone from the app's activity feed, but
  later edits to the same record block an undo, so be deliberate.
- General cooking knowledge is yours; the pairing tools only count the family's own recipes.
