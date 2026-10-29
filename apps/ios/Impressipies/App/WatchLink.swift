import Foundation
import WatchConnectivity

/// Hands the session to the paired watch, which has no sign-in of its own. The
/// latest server and token go out as the application context, so the watch
/// picks them up whenever it next wakes.
final class WatchLink: NSObject, WCSessionDelegate, @unchecked Sendable {
    static let shared = WatchLink()

    private let lock = NSLock()
    private var pending: [String: Any]?

    func start() {
        guard WCSession.isSupported() else { return }
        WCSession.default.delegate = self
        WCSession.default.activate()
    }

    /// A nil token signs the watch out.
    func share(server: URL, token: String?) {
        var context: [String: Any] = [WatchContext.server: server.absoluteString]
        if let token { context[WatchContext.token] = token }
        lock.withLock { pending = context }
        flush()
    }

    private func flush() {
        guard WCSession.isSupported() else { return }
        let session = WCSession.default
        guard session.activationState == .activated, session.isPaired, session.isWatchAppInstalled,
              let context = lock.withLock({ pending }) else { return }
        do {
            try session.updateApplicationContext(context)
            lock.withLock { pending = nil }
        } catch {}
    }

    func session(_ session: WCSession, activationDidCompleteWith state: WCSessionActivationState, error: Error?) {
        flush()
    }

    func sessionWatchStateDidChange(_ session: WCSession) {
        flush()
    }

    func sessionDidBecomeInactive(_ session: WCSession) {}

    func sessionDidDeactivate(_ session: WCSession) {
        // A different watch was paired; activate again so it gets the session.
        session.activate()
    }
}
