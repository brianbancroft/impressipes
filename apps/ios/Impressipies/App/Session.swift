import ApiClient
import Foundation
import Observation
import UIKit

/// A change pushed by the server's event stream.
struct ListEvent: Equatable, Decodable {
    let listId: String
    let itemId: String
    let version: Int64
}

/// Who is signed in, to which server, and the API client that goes with it.
@MainActor @Observable
final class Session {
    enum State: Equatable { case loading, signedOut, signedIn }

    private(set) var state: State = .loading
    private(set) var member: Member?
    private(set) var server: URL
    private(set) var api: API
    /// The latest item change from the event stream; list screens reload on it.
    private(set) var lastEvent: ListEvent?

    /// A message for the person that no screen asked for, such as a sign-in link that did not work.
    var notice: String?

    @ObservationIgnored private let tokenBox = TokenBox()
    @ObservationIgnored private let passkeys = PasskeyPrompt()
    @ObservationIgnored private var lastLinkToken: String?
    @ObservationIgnored private var eventsTask: Task<Void, Never>?

    private static let serverKey = "server"
    private static let tokenKey = "session"

    init() {
        let saved = UserDefaults.standard.string(forKey: Self.serverKey).flatMap(URL.init(string:))
        let server = saved ?? APIConfig.defaultServer
        self.server = server
        tokenBox.value = Keychain.read(Self.tokenKey)
        api = Self.makeAPI(server: server, box: tokenBox)
        tokenBox.onUnauthorized = { Task { @MainActor [weak self] in self?.expire() } }
        WatchLink.shared.start()
    }

    private static func makeAPI(server: URL, box: TokenBox) -> API {
        let middleware = SessionMiddleware(token: { box.value }, onUnauthorized: { box.onUnauthorized?() })
        return API(client: APIConfig.makeClient(server: server, middleware: middleware))
    }

    /// Checks a saved session on launch.
    func restore() async {
        guard tokenBox.value != nil else {
            state = .signedOut
            return
        }
        do {
            member = try await api.me()
            state = .signedIn
            startEvents()
            shareWithWatch()
        } catch let e as APIError where e.message == "unauthenticated" || e.message == "disabled" {
            signOut()
        } catch {
            // Offline or the server is down: stay signed in and let screens retry.
            state = .signedIn
            shareWithWatch()
        }
    }

    func setServer(_ url: URL) {
        server = url
        UserDefaults.standard.set(url.absoluteString, forKey: Self.serverKey)
        api = Self.makeAPI(server: url, box: tokenBox)
        shareWithWatch()
    }

    /// Signs in by accepting an invite link from the web app. The server only
    /// stores the credential id for now, so a device id stands in for it. An email,
    /// when given, is confirmed afterwards with the code the server mails.
    func acceptInvite(token: String, name: String, color: String, email: String? = nil, password: String? = nil) async throws {
        let device = "ios-" + UUID().uuidString.lowercased()
        let (m, sessionToken) = try await api.acceptInvite(
            token: token, name: name, color: color, credentialId: device, email: email, password: password
        )
        try adopt(m, token: sessionToken)
    }

    func signIn(email: String, password: String) async throws {
        let (m, token) = try await api.login(email: email, password: password)
        try adopt(m, token: token)
    }

    func requestLink(email: String) async throws {
        try await api.requestMagicLink(email: email)
    }

    /// Signs in (or confirms an email) with the six-digit code.
    func verify(email: String, code: String) async throws {
        let (m, token) = try await api.verify(email: email, code: code)
        try adopt(m, token: token)
    }

    /// Signs in with a passkey from iCloud Keychain. Does nothing if the person dismisses the sheet.
    func signInWithPasskey() async throws {
        let begin = try await api.beginPasskeyLogin()
        guard let credential = try await passkeys.signIn(options: begin.options) else { return }
        let (m, token) = try await api.finishPasskeyLogin(ceremonyId: begin.ceremonyId, credential: credential)
        try adopt(m, token: token)
    }

    func signInWithApple(identityToken: String, nonce: String) async throws {
        let (m, token) = try await api.signInWithApple(identityToken: identityToken, nonce: nonce)
        try adopt(m, token: token)
    }

    /// Adds a passkey for this member. Returns false if the person dismisses the sheet.
    func addPasskey() async throws -> Bool {
        let begin = try await api.beginPasskeyRegistration()
        guard let credential = try await passkeys.register(options: begin.options) else { return false }
        try await api.finishPasskeyRegistration(ceremonyId: begin.ceremonyId, credential: credential, name: UIDevice.current.model)
        return true
    }

    func setEmail(_ email: String) async throws {
        try await api.setEmail(email)
    }

    func setPassword(_ password: String) async throws {
        try await api.setPassword(password)
        member = try? await api.me()
    }

    /// Handles a tapped sign-in link. Returns false for URLs that are not one.
    @discardableResult
    func open(_ url: URL) async -> Bool {
        guard let link = AuthLink(url) else { return false }
        // The same link can arrive through two callbacks; a token only works once.
        guard link.token != lastLinkToken else { return true }
        lastLinkToken = link.token
        // Only this app's own server may sign the phone in.
        guard link.server == server || link.server == APIConfig.defaultServer else { return true }
        if link.server != server { setServer(link.server) }
        do {
            let (m, token) = try await api.verify(linkToken: link.token)
            try adopt(m, token: token)
        } catch {
            notice = APIError.from(error).message + ". Links work once and expire after 15 minutes."
        }
        return true
    }

    /// Takes a freshly issued session as the current one.
    private func adopt(_ m: Member, token sessionToken: String?) throws {
        guard let sessionToken else { throw APIError(message: "The server didn't start a session") }
        if let current = member, current.id != m.id {
            stopEvents()
            OfflineStore.shared.reset()
        }
        tokenBox.value = sessionToken
        Keychain.write(sessionToken, for: Self.tokenKey)
        member = m
        state = .signedIn
        startEvents()
        shareWithWatch()
    }

    func signOut() {
        stopEvents()
        tokenBox.value = nil
        OfflineStore.shared.reset()
        Keychain.write(nil, for: Self.tokenKey)
        member = nil
        state = .signedOut
        shareWithWatch()
    }

    /// The watch signs in and out with the phone.
    private func shareWithWatch() {
        WatchLink.shared.share(server: server, token: tokenBox.value)
    }

    private func expire() {
        if state == .signedIn { signOut() }
    }

    /// Absolute URL for a path the API hands back relative to its root, e.g. /images/…/thumb.
    func apiURL(_ path: String) -> URL {
        URL(string: APIConfig.apiRoot(for: server).absoluteString + path)!
    }

    /// Loads image bytes that need the session cookie.
    func imageData(_ path: String) async throws -> Data {
        var req = URLRequest(url: apiURL(path))
        if let t = tokenBox.value { req.setValue("\(APIConfig.sessionCookie)=\(t)", forHTTPHeaderField: "Cookie") }
        let (data, resp) = try await URLSession.shared.data(for: req)
        guard (resp as? HTTPURLResponse)?.statusCode == 200 else { throw APIError(message: "Image unavailable") }
        return data
    }

    // MARK: Live updates

    func startEvents() {
        guard state == .signedIn, eventsTask == nil else { return }
        let url = apiURL("/events")
        let token = tokenBox.value
        eventsTask = Task { [weak self] in
            while !Task.isCancelled {
                do {
                    var req = URLRequest(url: url)
                    req.timeoutInterval = 60 * 60
                    req.setValue("text/event-stream", forHTTPHeaderField: "Accept")
                    req.setValue(APIConfig.clientTag, forHTTPHeaderField: "X-Impressipies-Client")
                    if let token { req.setValue("\(APIConfig.sessionCookie)=\(token)", forHTTPHeaderField: "Cookie") }
                    let (bytes, resp) = try await URLSession.shared.bytes(for: req)
                    guard (resp as? HTTPURLResponse)?.statusCode == 200 else { throw URLError(.badServerResponse) }
                    for try await line in bytes.lines {
                        guard line.hasPrefix("data:") else { continue }
                        let json = line.dropFirst(5).trimmingCharacters(in: .whitespaces)
                        if let e = try? JSONDecoder().decode(ListEvent.self, from: Data(json.utf8)) {
                            await MainActor.run { self?.lastEvent = e }
                        }
                    }
                } catch {}
                // The stream dropped; the server asks for a 3 s retry.
                try? await Task.sleep(for: .seconds(3))
            }
        }
    }

    func stopEvents() {
        eventsTask?.cancel()
        eventsTask = nil
    }
}

/// The token, readable from the generated client's (non-isolated) middleware.
private final class TokenBox: @unchecked Sendable {
    private let lock = NSLock()
    private var _value: String?
    var onUnauthorized: (@Sendable () -> Void)?
    var value: String? {
        get { lock.withLock { _value } }
        set { lock.withLock { _value = newValue } }
    }
}
