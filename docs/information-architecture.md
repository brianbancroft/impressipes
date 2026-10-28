# Information architecture & navigation

Issue: IMP-2 (Notion). Sources: PRDs "Grocery lists (iPhone)" (M1) and "Recipe library (web)" (M2).

## Principle: who lives where

| Surface | Primary job | Secondary |
| --- | --- | --- |
| iOS | Lists: the in-store daily driver (offline-first, live sync) | Browse and cook from recipes, add recipe ingredients to a list |
| Web | Recipes: browse, edit, cook mode (laptop, iPad on the counter, phone browser, 360 px+) | Full list view and editing |
| Watch | Glanceable list: check items off in the store | Nothing else |

Recipe authoring and editing is web-first. Importing from URLs and photos is done by AI over MCP, so no surface has an importer.

## iOS

Tab bar, three tabs:

```
Lists (default tab)
  All lists (emoji + colour, unchecked count)
    List detail
      Add-item field (quantity in one line, autocomplete from item memory)
      Category sections (coloured, labelled)
      "Got it" section (checked, undo)
      Clear checked
      List settings (name, emoji, colour, categories, members)
  New list
Recipes
  Recipe list (search, tag filter, favourites / want to make)
    Recipe detail (ingredients, steps, servings scaler)
      Add ingredients to list (pick list, scaled quantities)
      Cook mode
Settings
  Account and family members
  Appearance (light / dark)
  Sync status
```

Notes:
- Lists is the launch tab and the only tab needed for M1. Recipes arrives with M2 and stays read and cook only on iOS (manual edit is a web action).
- Settings is a tab because it is small and keeps room for the sync status. If it stays thin, demote it to a gear icon on the Lists screen and make it two tabs.

## Web

Top nav (collapses to a hamburger or bottom bar under 640 px):

```
/                        Home: redirects to /recipes
/recipes                 Browse cards, search (name / ingredient), tag filter, favourites
/recipes/new             Manual add
/recipes/:id             Recipe: ingredients, steps, servings scaler, source and added-by
/recipes/:id/edit        Manual edit
/recipes/:id/cook        Cook mode: big type, screen awake, tap steps, timers
/lists                   All lists
/lists/:id               List detail (same model as iOS)
/settings                Account, family, theme
```

Notes:
- Cook mode is its own route so it can drop the nav chrome and be bookmarked or opened on the iPad.
- "Add ingredients to list" is a dialog on `/recipes/:id`, not a route.

## Watch

```
Lists                    All lists
  List                   Unchecked items by category colour, tap to check off
```

Two levels only. No adding, recipes or settings on the watch; add by voice or from the phone.

## Shared concepts

- A list's colour and emoji identify it everywhere. Category colour is always paired with a label or icon for accessibility.
- Recipe to list is the one cross-over: "Add to list" scales quantities and guesses a category from item memory.
- Same data, same names across surfaces: Lists, Recipes, "Got it", Cook mode.

## Mobbin references

Mobbin had few close matches for shared shopping lists or recipe boxes, so these ground individual decisions rather than the whole structure.

- **Lists screen:** [Noom's grocery list](https://mobbin.com/screens/46d58d6e-4c04-4f51-aa49-2c5d9f1f439a) has an add field at the top, checked items struck through below unchecked, a "Clear list" action and an auto-save note. That matches our list detail and "Got it" section.
- **Recipes inside a tab:** [MacroFactor's library](https://mobbin.com/screens/9709d49f-1ed5-49de-b3de-f2e0344489aa) uses a segmented control (Recipes / Foods / favourites) with a filter field at the bottom, and [Lifesum's favourites](https://mobbin.com/screens/ff1fa58b-09db-4b96-9708-6ca980be542d) uses top tabs with a sort toggle. Both support putting favourites and "want to make" as filters inside Recipes rather than as a tab.
- **Tab bar:** [Rakuten](https://mobbin.com/screens/32f13fec-1b36-417a-88ce-c3a725353bb8) and [Noom](https://mobbin.com/screens/46d58d6e-4c04-4f51-aa49-2c5d9f1f439a) keep to three to five labelled tabs, which supports our three.
- **Web:** the web results were mostly dashboard sidebars (for example [Sprig](https://mobbin.com/screens/1a4211a1-0854-4295-aac5-82d8cc85bc0c)). That suits admin tools more than a family recipe box, so the top nav above stands.

## Open questions

- Settings as a third iOS tab or a gear icon.
- Whether web's landing page should be Recipes (as above) or a lists overview, given lists are mostly used on the phone.
- Whether the watch needs a complication that opens straight to the Main list.
