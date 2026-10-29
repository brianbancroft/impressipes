import Foundation

/// The emailed sign-in link: https://<server>/auth/verify?t=<token>.
struct AuthLink: Equatable {
    let server: URL
    let token: String

    init?(_ url: URL) {
        guard let c = URLComponents(url: url, resolvingAgainstBaseURL: false),
              let scheme = c.scheme, scheme == "https" || scheme == "http",
              let host = c.host, c.path == "/auth/verify",
              let t = c.queryItems?.first(where: { $0.name == "t" })?.value, !t.isEmpty
        else { return nil }
        var origin = URLComponents()
        origin.scheme = scheme
        origin.host = host
        origin.port = c.port
        guard let server = origin.url else { return nil }
        self.server = server
        self.token = t
    }
}
