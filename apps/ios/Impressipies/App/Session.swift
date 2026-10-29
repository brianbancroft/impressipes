import ApiClient
import Foundation
import Observation

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

    @ObservationIgnored private let tokenBox = TokenBox()
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
    /// stores the credential id for now (passkey sign-in is still to come), so a
    /// device id stands in for it.
    func acceptInvite(token: String, name: String, color: String) async throws {
        let device = "ios-" + UUID().uuidString.lowercased()
        let (m, sessionToken) = try await api.acceptInvite(token: token, name: name, color: color, credentialId: device)
        guard let sessionToken else { throw APIError(message: "The server didn't start a session") }
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
