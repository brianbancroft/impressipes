import ApiClient
import Foundation

typealias Member = Components.Schemas.Member
typealias ShoppingList = Components.Schemas.List
typealias ListDetail = Components.Schemas.ListDetail
typealias Category = Components.Schemas.Category
typealias Item = Components.Schemas.Item
typealias Suggestion = Components.Schemas.Suggestion
typealias RecipeSummary = Components.Schemas.RecipeSummary
typealias Recipe = Components.Schemas.Recipe
typealias RecipeMark = Components.Schemas.RecipeMark
typealias Ingredient = Components.Schemas.Ingredient
typealias Step = Components.Schemas.Step
typealias AddRecipeResult = Components.Schemas.AddRecipeResult
typealias InviteInfo = Components.Schemas.InviteInfo
typealias CategoryInput = Components.Schemas.CategoryInput
typealias Passkey = Components.Schemas.Passkey
typealias SignInMethods = Components.Schemas.SignInMethods
typealias AuthConfig = Components.Schemas.AuthConfig

/// A passkey ceremony the server started: its id, and the WebAuthn options as JSON.
struct PasskeyChallenge: Sendable {
    let ceremonyId: String
    let options: Data
}

/// The calls the iOS app makes, as plain async functions. Recipe authoring is a
/// web job, so there is nothing here that writes a recipe beyond its marks.
struct API: Sendable {
    let client: Client

    private func call<T>(_ f: () async throws -> T) async throws -> T {
        do { return try await f() } catch { throw APIError.from(error) }
    }

    // MARK: Account

    func me() async throws -> Member {
        try await call { try await client.getMe().ok.body.json }
    }

    /// What this server offers on the sign-in screen. Also tells us the address is an Impressipes server.
    func authConfig() async throws -> AuthConfig {
        try await call { try await client.getAuthConfig().ok.body.json }
    }

    func invite(token: String) async throws -> InviteInfo {
        try await call {
            switch try await client.getInvite(path: .init(token: token)) {
            case let .ok(r): return try r.body.json
            case let .notFound(r): return try r.body.json
            case let .undocumented(code, _): throw APIError.status(code)
            }
        }
    }

    /// Accepts an invite and returns the member plus the new session token. A given
    /// email is confirmed afterwards with the link or code the server mails.
    func acceptInvite(token: String, name: String, color: String, credentialId: String, email: String? = nil, password: String? = nil) async throws -> (Member, String?) {
        try await call {
            let created = try await client.acceptInvite(
                path: .init(token: token),
                body: .json(.init(name: name, color: color, credentialId: credentialId, email: email, password: password))
            ).created
            return (try created.body.json, created.headers.Set_hyphen_Cookie.flatMap(Self.sessionToken))
        }
    }

    /// Signs in with a confirmed email and a password.
    func login(email: String, password: String) async throws -> (Member, String?) {
        try await call {
            let ok = try await client.login(body: .json(.init(email: email, password: password))).ok
            return (try ok.body.json, ok.headers.Set_hyphen_Cookie.flatMap(Self.sessionToken))
        }
    }

    /// Emails a link and six-digit code. The server answers the same for any address.
    func requestMagicLink(email: String) async throws {
        try await call { _ = try await client.requestMagicLink(body: .json(.init(email: email))).accepted }
    }

    /// Redeems the token from an emailed link.
    func verify(linkToken: String) async throws -> (Member, String?) {
        try await verify(.init(token: linkToken))
    }

    /// Redeems the six-digit code from the latest email to this address.
    func verify(email: String, code: String) async throws -> (Member, String?) {
        try await verify(.init(email: email, code: code))
    }

    private func verify(_ body: Operations.verifyMagicLink.Input.Body.jsonPayload) async throws -> (Member, String?) {
        try await call {
            let ok = try await client.verifyMagicLink(body: .json(body)).ok
            return (try ok.body.json, ok.headers.Set_hyphen_Cookie.flatMap(Self.sessionToken))
        }
    }

    /// Emails a confirmation link and code to a new address.
    func setEmail(_ email: String) async throws {
        try await call { _ = try await client.setEmail(body: .json(.init(email: email))).accepted }
    }

    func setPassword(_ password: String) async throws {
        try await call { _ = try await client.setPassword(body: .json(.init(password: password))).noContent }
    }

    // MARK: Passkeys and Apple

    func beginPasskeyLogin() async throws -> PasskeyChallenge {
        try await call { try Self.challenge(from: try await client.beginPasskeyLogin().ok.body.json) }
    }

    /// Finishes a passkey sign-in. `credential` is the WebAuthn JSON the device produced.
    func finishPasskeyLogin(ceremonyId: String, credential: Data) async throws -> (Member, String?) {
        try await call {
            let ok = try await client.finishPasskeyLogin(body: .json(try Self.finish(ceremonyId, credential))).ok
            return (try ok.body.json, ok.headers.Set_hyphen_Cookie.flatMap(Self.sessionToken))
        }
    }

    /// Signs in with the identity token Apple handed back. `nonce` is the raw value whose SHA-256 went to Apple.
    func signInWithApple(identityToken: String, nonce: String) async throws -> (Member, String?) {
        try await call {
            let ok = try await client.signInWithApple(body: .json(.init(identityToken: identityToken, nonce: nonce))).ok
            return (try ok.body.json, ok.headers.Set_hyphen_Cookie.flatMap(Self.sessionToken))
        }
    }

    func signInMethods() async throws -> SignInMethods {
        try await call { try await client.getSignInMethods().ok.body.json }
    }

    func beginPasskeyRegistration() async throws -> PasskeyChallenge {
        try await call { try Self.challenge(from: try await client.beginPasskeyRegistration().ok.body.json) }
    }

    func finishPasskeyRegistration(ceremonyId: String, credential: Data, name: String) async throws {
        try await call {
            _ = try await client.finishPasskeyRegistration(
                body: .json(.init(value1: try Self.finish(ceremonyId, credential), value2: .init(name: name)))
            ).created
        }
    }

    func deletePasskey(_ id: String) async throws {
        try await call { _ = try await client.deletePasskey(path: .init(passkeyId: id)).noContent }
    }

    func linkApple(identityToken: String, nonce: String) async throws {
        try await call { _ = try await client.linkApple(body: .json(.init(identityToken: identityToken, nonce: nonce))).ok }
    }

    func unlinkApple() async throws {
        try await call { _ = try await client.unlinkApple().noContent }
    }

    private static func challenge(from c: Components.Schemas.PasskeyCeremony) throws -> PasskeyChallenge {
        PasskeyChallenge(ceremonyId: c.ceremonyId, options: try JSONEncoder().encode(c.options))
    }

    private static func finish(_ ceremonyId: String, _ credential: Data) throws -> Components.Schemas.PasskeyFinish {
        guard let object = try JSONSerialization.jsonObject(with: credential) as? [String: Any] else {
            throw APIError(message: "The passkey response was unreadable")
        }
        let container = try OpenAPIObjectContainer(unvalidatedValue: object)
        return .init(ceremonyId: ceremonyId, credential: .init(additionalProperties: container))
    }

    /// Pulls the session value out of a Set-Cookie header.
    static func sessionToken(fromSetCookie header: String) -> String? {
        for part in header.split(separator: ";") {
            let kv = part.trimmingCharacters(in: .whitespaces).split(separator: "=", maxSplits: 1)
            if kv.count == 2, kv[0] == APIConfig.sessionCookie { return String(kv[1]) }
        }
        return nil
    }

    // MARK: Lists

    func lists() async throws -> [ShoppingList] {
        try await call { try await client.listLists().ok.body.json }
    }

    func list(_ id: String) async throws -> ListDetail {
        try await call { try await client.getList(path: .init(listId: id)).ok.body.json }
    }

    func createList(name: String, emoji: String, colour: String, categories: [CategoryInput]) async throws -> ShoppingList {
        try await call {
            try await client.createList(body: .json(.init(name: name, emoji: emoji, colour: colour, categories: categories))).created.body.json
        }
    }

    func updateList(_ id: String, name: String, emoji: String, colour: String) async throws -> ShoppingList {
        try await call {
            try await client.updateList(path: .init(listId: id), body: .json(.init(name: name, emoji: emoji, colour: colour))).ok.body.json
        }
    }

    func archiveList(_ id: String) async throws {
        try await call { _ = try await client.archiveList(path: .init(listId: id)).noContent }
    }

    // MARK: Items

    func createItem(listId: String, id: String = UUID().uuidString.lowercased(), name: String, quantityText: String?, note: String? = nil, categoryId: String?) async throws -> Item {
        try await call {
            try await client.createItem(
                path: .init(listId: listId),
                body: .json(.init(id: id, name: name, quantityText: quantityText, note: note, categoryId: categoryId))
            ).created.body.json
        }
    }

    func updateItem(_ id: String, _ patch: Components.Schemas.ItemPatch) async throws -> Item {
        try await call { try await client.updateItem(path: .init(itemId: id), body: .json(patch)).ok.body.json }
    }

    func deleteItem(_ id: String) async throws {
        try await call { _ = try await client.deleteItem(path: .init(itemId: id)).noContent }
    }

    func suggestions(listId: String, query: String) async throws -> [Suggestion] {
        try await call { try await client.suggestItems(path: .init(listId: listId), query: .init(q: query)).ok.body.json }
    }

    // MARK: Recipes

    func recipes(query: String?, tag: String?, mark: RecipeMark?) async throws -> [RecipeSummary] {
        try await call {
            try await client.listRecipes(query: .init(q: query, tag: tag, mark: mark)).ok.body.json
        }
    }

    func tags() async throws -> [String] {
        try await call { try await client.listTags().ok.body.json }
    }

    func recipe(_ id: String) async throws -> Recipe {
        try await call { try await client.getRecipe(path: .init(recipeId: id)).ok.body.json }
    }

    func setMark(_ mark: RecipeMark, on recipeId: String, _ on: Bool) async throws {
        try await call {
            if on {
                _ = try await client.markRecipe(path: .init(recipeId: recipeId, mark: mark)).noContent
            } else {
                _ = try await client.unmarkRecipe(path: .init(recipeId: recipeId, mark: mark)).noContent
            }
        }
    }

    func addRecipe(_ recipeId: String, to listId: String, servings: Double?) async throws -> AddRecipeResult {
        try await call {
            try await client.addRecipeToList(path: .init(listId: listId), body: .json(.init(recipeId: recipeId, servings: servings))).ok.body.json
        }
    }
}
