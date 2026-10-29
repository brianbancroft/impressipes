import ApiClient
import Foundation
import Observation
import WatchConnectivity

/// The session the phone handed over, and the API client that goes with it.
/// The watch has no sign-in of its own: it waits for the phone.
@MainActor @Observable
final class WatchSession {
    enum State: Equatable { case waiting, signedIn }

    private(set) var state: State
    private(set) var api: API

    @ObservationIgnored private let tokenBox = WatchTokenBox()
    @ObservationIgnored private let link = PhoneLink()

    private static let serverKey = "server"
    private static let tokenKey = "session"

    init() {
        var server = UserDefaults.standard.string(forKey: Self.serverKey).flatMap(URL.init(string:)) ?? APIConfig.defaultServer
        tokenBox.value = Keychain.read(Self.tokenKey)
        #if DEBUG
        // Runs the watch without a paired phone in the simulator:
        // -debugServer http://localhost:8080 -debugToken <session id>
        if let token = UserDefaults.standard.string(forKey: "debugToken") {
            tokenBox.value = token
            server = UserDefaults.standard.string(forKey: "debugServer").flatMap(URL.init(string:)) ?? server
        }
        #endif
        state = tokenBox.value == nil ? .waiting : .signedIn
        api = Self.makeAPI(server: server, box: tokenBox)
        tokenBox.onUnauthorized = { Task { @MainActor [weak self] in self?.apply(server: nil, token: nil) } }
        link.onContext = { server, token in
            Task { @MainActor [weak self] in self?.apply(server: server, token: token) }
        }
        link.start()
    }

    private static func makeAPI(server: URL, box: WatchTokenBox) -> API {
        let middleware = SessionMiddleware(token: { box.value }, onUnauthorized: { box.onUnauthorized?() })
        return API(client: APIConfig.makeClient(server: server, middleware: middleware))
    }

    /// Takes the server and token from the phone. A nil token signs out.
    private func apply(server: String?, token: String?) {
        if let server, let url = URL(string: server) {
            UserDefaults.standard.set(server, forKey: Self.serverKey)
            api = Self.makeAPI(server: url, box: tokenBox)
        }
        tokenBox.value = token
        Keychain.write(token, for: Self.tokenKey)
        state = token == nil ? .waiting : .signedIn
    }
}

/// Receives the phone's application context.
private final class PhoneLink: NSObject, WCSessionDelegate, @unchecked Sendable {
    var onContext: (@Sendable (String?, String?) -> Void)?

    func start() {
        guard WCSession.isSupported() else { return }
        WCSession.default.delegate = self
        WCSession.default.activate()
    }

    private func deliver(_ context: [String: Any]) {
        guard !context.isEmpty else { return }
        onContext?(context[WatchContext.server] as? String, context[WatchContext.token] as? String)
    }

    func session(_ session: WCSession, activationDidCompleteWith state: WCSessionActivationState, error: Error?) {
        // The last context sent while the watch app was closed.
        deliver(session.receivedApplicationContext)
    }

    func session(_ session: WCSession, didReceiveApplicationContext context: [String: Any]) {
        deliver(context)
    }
}

/// The token, readable from the generated client's (non-isolated) middleware.
private final class WatchTokenBox: @unchecked Sendable {
    private let lock = NSLock()
    private var _value: String?
    var onUnauthorized: (@Sendable () -> Void)?
    var value: String? {
        get { lock.withLock { _value } }
        set { lock.withLock { _value = newValue } }
    }
}
