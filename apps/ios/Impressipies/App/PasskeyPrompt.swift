import AuthenticationServices
import UIKit

/// Shows the system passkey sheet (Face ID, iCloud Keychain) and returns what the
/// authenticator produced in WebAuthn's JSON form, ready for the server.
///
/// Passkeys are tied to the site's domain, so the app needs the
/// `webcredentials:<domain>` associated domain, and the server must list the app in
/// its apple-app-site-association file (see APPLE_TEAM_ID).
@MainActor
final class PasskeyPrompt: NSObject, ASAuthorizationControllerDelegate, ASAuthorizationControllerPresentationContextProviding {
    private var continuation: CheckedContinuation<ASAuthorization, Error>?
    private var controller: ASAuthorizationController?

    /// Signs in with a passkey the device holds. Returns nil when the person dismisses the sheet.
    func signIn(options: Data) async throws -> Data? {
        let pk = try Self.publicKey(options)
        guard let challenge = Data(base64URL: pk["challenge"] as? String), let rpID = pk["rpId"] as? String else {
            throw APIError(message: "The server's passkey request was unreadable")
        }
        let request = ASAuthorizationPlatformPublicKeyCredentialProvider(relyingPartyIdentifier: rpID)
            .createCredentialAssertionRequest(challenge: challenge)
        request.userVerificationPreference = .required
        guard let auth = try await perform(request),
              let c = auth.credential as? ASAuthorizationPlatformPublicKeyCredentialAssertion else { return nil }
        return try JSONSerialization.data(withJSONObject: [
            "id": c.credentialID.base64URL, "rawId": c.credentialID.base64URL, "type": "public-key",
            "response": [
                "clientDataJSON": c.rawClientDataJSON.base64URL,
                "authenticatorData": c.rawAuthenticatorData.base64URL,
                "signature": c.signature.base64URL,
                "userHandle": c.userID.base64URL,
            ],
        ])
    }

    /// Creates a passkey for the signed-in member. Returns nil when the person dismisses the sheet.
    func register(options: Data) async throws -> Data? {
        let pk = try Self.publicKey(options)
        guard let challenge = Data(base64URL: pk["challenge"] as? String),
              let rp = pk["rp"] as? [String: Any], let rpID = rp["id"] as? String,
              let user = pk["user"] as? [String: Any], let userID = Data(base64URL: user["id"] as? String),
              let name = user["name"] as? String else {
            throw APIError(message: "The server's passkey request was unreadable")
        }
        let request = ASAuthorizationPlatformPublicKeyCredentialProvider(relyingPartyIdentifier: rpID)
            .createCredentialRegistrationRequest(challenge: challenge, name: name, userID: userID)
        request.displayName = user["displayName"] as? String
        request.userVerificationPreference = .required
        guard let auth = try await perform(request),
              let c = auth.credential as? ASAuthorizationPlatformPublicKeyCredentialRegistration,
              let attestation = c.rawAttestationObject else { return nil }
        return try JSONSerialization.data(withJSONObject: [
            "id": c.credentialID.base64URL, "rawId": c.credentialID.base64URL, "type": "public-key",
            "response": [
                "clientDataJSON": c.rawClientDataJSON.base64URL,
                "attestationObject": attestation.base64URL,
                "transports": ["internal", "hybrid"],
            ],
        ])
    }

    private static func publicKey(_ options: Data) throws -> [String: Any] {
        guard let root = try JSONSerialization.jsonObject(with: options) as? [String: Any],
              let pk = root["publicKey"] as? [String: Any] else {
            throw APIError(message: "The server's passkey request was unreadable")
        }
        return pk
    }

    private func perform(_ request: ASAuthorizationRequest) async throws -> ASAuthorization? {
        do {
            return try await withCheckedThrowingContinuation { (cont: CheckedContinuation<ASAuthorization, Error>) in
                continuation = cont
                let c = ASAuthorizationController(authorizationRequests: [request])
                c.delegate = self
                c.presentationContextProvider = self
                controller = c
                c.performRequests()
            }
        } catch let e as ASAuthorizationError where e.code == .canceled {
            return nil
        } catch let e as ASAuthorizationError where e.code == .failed || e.code == .notHandled {
            throw APIError(message: "This phone couldn't use a passkey for Impressipes. Sign in another way.")
        }
    }

    nonisolated func authorizationController(controller: ASAuthorizationController, didCompleteWithAuthorization authorization: ASAuthorization) {
        Task { @MainActor in
            continuation?.resume(returning: authorization)
            continuation = nil
        }
    }

    nonisolated func authorizationController(controller: ASAuthorizationController, didCompleteWithError error: Error) {
        Task { @MainActor in
            continuation?.resume(throwing: error)
            continuation = nil
        }
    }

    nonisolated func presentationAnchor(for controller: ASAuthorizationController) -> ASPresentationAnchor {
        MainActor.assumeIsolated {
            UIApplication.shared.connectedScenes
                .compactMap { $0 as? UIWindowScene }
                .flatMap(\.windows)
                .first(where: \.isKeyWindow) ?? ASPresentationAnchor()
        }
    }
}

extension Data {
    /// WebAuthn's JSON carries binary values as unpadded base64url.
    init?(base64URL string: String?) {
        guard var s = string else { return nil }
        s = s.replacingOccurrences(of: "-", with: "+").replacingOccurrences(of: "_", with: "/")
        s += String(repeating: "=", count: (4 - s.count % 4) % 4)
        self.init(base64Encoded: s)
    }

    var base64URL: String {
        base64EncodedString().replacingOccurrences(of: "+", with: "-").replacingOccurrences(of: "/", with: "_").replacingOccurrences(of: "=", with: "")
    }
}
