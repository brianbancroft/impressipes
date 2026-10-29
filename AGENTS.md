# Agent guide

## Issue tracking

Issue IDs prefixed with `IMP-` (e.g. `IMP-1`) are issues in the Notion issue tracker (the **Issues** database under the **Impressipies** page), not Linear. Look them up in Notion by the `ID` property.

## Deploying

There is no auto-deploy. To deploy, run `mise run deploy` (see `docs/deploy.md`); it pushes the current branch and triggers Coolify. The app is `impressipes` on Coolify, live at https://impressipes.bancroft.io.
