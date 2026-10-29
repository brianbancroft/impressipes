# Impressipes for iOS

SwiftUI app for iPhone (iOS 18+). It reads recipes and runs shopping lists:

- **Lists**: make, rename, recolour and archive lists; quick-add items ("2 cans chickpeas"), add with details (quantity, aisle, note), edit, tick off, delete with undo, clear the basket. Changes from other people arrive live over the API's event stream.
- **Recipes**: browse, search, filter by faves, want to make, under 30 minutes and tags; read ingredients and steps with a servings scaler; mark favourites or want to make; add a recipe's ingredients to a list, scaled.
- No recipe authoring: that stays on the web.

## Watch

The watch app (`ImpressipiesWatch`, IMP-20 designs in `designs/imp-20`) is always dark and does two things: pick a list, then cross items off and put them back from "Got it". There is no adding, editing or sign-in on the watch. The phone hands its server and session token over WatchConnectivity whenever it signs in or out, and the watch waits on "Sign in on your iPhone" until then. Lists refresh on open, on wake and every 30 s while a list is open.

Out of reach of the phone and signal is a normal state (IMP-58). Check-offs apply at once and join a queue on disk (`OfflineStore`), which is sent whenever the server can be reached; rows still waiting show a clock glyph. The last-seen lists are cached too, so the watch opens with content, shows "Updated hh:mm", and adds an "Offline" line after 10 minutes without a successful sync. There are no error screens for being unreachable. The watch talks to the API directly with the token the phone handed over, so it needs no live phone connection.

The API layer in `Shared/` is compiled into both apps; the watch tags its requests as `watch`.

To run the watch on its own in the simulator (Debug builds only), pass a server and a session id as launch arguments:

```bash
xcrun simctl launch <watch-udid> ca.brianbancroft.impressipies.watchkitapp -debugServer http://localhost:8080 -debugToken <session id>
```

## Running

Open `Impressipies.xcodeproj` and run the `Impressipies` scheme, or:

```bash
xcodebuild -project apps/ios/Impressipies.xcodeproj -scheme Impressipies \
  -destination "platform=iOS Simulator,name=iPhone 17" -derivedDataPath apps/ios/build build
```

The Xcode project is generated from `project.yml` with [XcodeGen](https://github.com/yonaskolb/XcodeGen) (`brew install xcodegen`); edit the spec, not the `.xcodeproj`, and run `nx xcodegen ios` to regenerate. `nx build ios` and `nx test ios` regenerate it first, then run `xcodebuild`. `Shared/` is compiled into both apps and stands in for the shared kit (models, API client, keychain).

The project uses synced folders, so new files under `Impressipies/` are picked up without editing the project. API calls (in `Shared/API`) go through the generated client in `libs/api-spec/swift` (`ApiClient`), so regenerate it with `mise run codegen` after changing `openapi.yaml`.

## Signing in

There is no passkey sign-in on the server yet (IMP-29), so the app signs in by accepting an invite link, the same way the web onboarding does:

1. On the web, an admin creates an invite (or a re-invite for an existing member to add this phone).
2. Paste the link into the app. The link's origin picks the server; a `localhost:4200` link uses the API on `localhost:8080`.
3. The session token is kept in the keychain and sent as the `impressipies_session` cookie.

Accepting a re-invite replaces that member's stored passkeys, as it does on the web.
