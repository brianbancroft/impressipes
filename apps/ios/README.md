# Impressipes for iOS

SwiftUI app for iPhone (iOS 18+). It reads recipes and runs shopping lists:

- **Lists**: make, rename, recolour and archive lists; quick-add items ("2 cans chickpeas"), add with details (quantity, aisle, note), edit, tick off, delete with undo, clear the basket. Changes from other people arrive live over the API's event stream.
- **Recipes**: browse, search, filter by faves, want to make, under 30 minutes and tags; read ingredients and steps with a servings scaler; mark favourites or want to make; add a recipe's ingredients to a list, scaled.
- No recipe authoring: that stays on the web.

The watch target is still a placeholder (IMP-20).

## Running

Open `Impressipies.xcodeproj` and run the `Impressipies` scheme, or:

```bash
xcodebuild -project apps/ios/Impressipies.xcodeproj -scheme Impressipies \
  -destination "platform=iOS Simulator,name=iPhone 17" -derivedDataPath apps/ios/build build
```

The project uses synced folders, so new files under `Impressipies/` are picked up without editing the project. API calls go through the generated client in `libs/api-spec/swift` (`ApiClient`), so regenerate it with `mise run codegen` after changing `openapi.yaml`.

## Signing in

There is no passkey sign-in on the server yet (IMP-29), so the app signs in by accepting an invite link, the same way the web onboarding does:

1. On the web, an admin creates an invite (or a re-invite for an existing member to add this phone).
2. Paste the link into the app. The link's origin picks the server; a `localhost:4200` link uses the API on `localhost:8080`.
3. The session token is kept in the keychain and sent as the `impressipies_session` cookie.

Accepting a re-invite replaces that member's stored passkeys, as it does on the web.
