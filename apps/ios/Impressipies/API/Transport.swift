import ApiClient
import Foundation

/// Adds the session cookie and client tag to every request, and reports a 401 so
/// the app can return to sign-in.
struct SessionMiddleware: ClientMiddleware {
    let token: @Sendable () -> String?
    let onUnauthorized: @Sendable () -> Void

    func intercept(
        _ request: HTTPRequest,
        body: HTTPBody?,
        baseURL: URL,
        operationID: String,
        next: (HTTPRequest, HTTPBody?, URL) async throws -> (HTTPResponse, HTTPBody?)
    ) async throws -> (HTTPResponse, HTTPBody?) {
        var request = request
        // Recorded as "ios" in the household activity feed.
        request.headerFields[HTTPField.Name("X-Impressipies-Client")!] = "ios"
        if let token = token() {
            request.headerFields[.cookie] = "\(APIConfig.sessionCookie)=\(token)"
        }
        let (response, responseBody) = try await next(request, body, baseURL)
        if response.status == .unauthorized, operationID != "acceptInvite" {
            onUnauthorized()
        }
        // Turn error bodies into an APIError carrying the server's message, so
        // callers can use the generated `.ok` accessors and still show why.
        // getInvite answers 404 with an InviteInfo body, so it is left alone.
        if response.status.code >= 400, operationID != "getInvite" {
            var message = APIError.status(response.status.code).message
            if let responseBody,
               let data = try? await Data(collecting: responseBody, upTo: 64 * 1024),
               let e = try? JSONDecoder().decode(Components.Schemas._Error.self, from: data) {
                message = e.error
            }
            throw APIError(message: message)
        }
        return (response, responseBody)
    }
}

/// Go writes RFC 3339 with fractional seconds only when they are non-zero, so
/// accept both forms.
struct FlexibleDateTranscoder: DateTranscoder {
    func encode(_ date: Date) throws -> String {
        date.formatted(.iso8601.year().month().day().time(includingFractionalSeconds: true).timeZone(separator: .omitted))
    }

    func decode(_ string: String) throws -> Date {
        if let d = try? Date(string, strategy: .iso8601.year().month().day().time(includingFractionalSeconds: true).timeZone(separator: .omitted)) {
            return d
        }
        if let d = try? Date(string, strategy: .iso8601) {
            return d
        }
        throw DecodingError.dataCorrupted(.init(codingPath: [], debugDescription: "Bad date: \(string)"))
    }
}

enum APIConfig {
    static let sessionCookie = "impressipies_session"
    static let defaultServer = URL(string: "https://impressipes.bancroft.io")!

    /// The API lives under /api on the same origin as the web app.
    static func apiRoot(for server: URL) -> URL {
        server.appending(path: "api")
    }

    static func makeClient(server: URL, middleware: SessionMiddleware) -> Client {
        Client(
            serverURL: apiRoot(for: server),
            configuration: .init(dateTranscoder: FlexibleDateTranscoder()),
            transport: URLSessionTransport(configuration: .init(session: .shared)),
            middlewares: [middleware]
        )
    }
}

/// An error worth showing to a person, pulled out of the API's error bodies.
struct APIError: LocalizedError {
    let message: String
    var errorDescription: String? { message }

    static func status(_ code: Int) -> APIError {
        APIError(message: code == 401 ? "You're signed out" : "The server answered \(code)")
    }

    /// Unwraps the generated client's error into something readable.
    static func from(_ error: Error) -> APIError {
        if let e = error as? APIError { return e }
        if let e = error as? ClientError { return from(e.underlyingError) }
        if let e = error as? URLError {
            switch e.code {
            case .notConnectedToInternet, .networkConnectionLost:
                return APIError(message: "You're offline")
            case .cannotConnectToHost, .cannotFindHost, .timedOut:
                return APIError(message: "Can't reach the server")
            default:
                return APIError(message: e.localizedDescription)
            }
        }
        return APIError(message: error.localizedDescription)
    }
}
