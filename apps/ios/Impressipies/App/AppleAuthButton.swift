import AuthenticationServices
import CryptoKit
import SwiftUI

/// A nonce for Sign in with Apple: the raw value goes to our server, and its SHA-256
/// goes to Apple so the identity token can only be used by whoever made the request.
struct AppleNonce {
    let raw = UUID().uuidString

    var hashed: String {
        SHA256.hash(data: Data(raw.utf8)).map { String(format: "%02x", $0) }.joined()
    }
}

/// Apple's own button. `action` gets the identity token and the raw nonce.
struct AppleAuthButton: View {
    let label: SignInWithAppleButton.Label
    let onError: (String) -> Void
    let action: (_ identityToken: String, _ nonce: String) async -> Void

    @Environment(\.colorScheme) private var scheme
    @State private var nonce = AppleNonce()

    var body: some View {
        SignInWithAppleButton(label) { request in
            nonce = AppleNonce()
            request.requestedScopes = [.email]
            request.nonce = nonce.hashed
        } onCompletion: { result in
            switch result {
            case let .success(auth):
                guard let credential = auth.credential as? ASAuthorizationAppleIDCredential,
                      let data = credential.identityToken, let token = String(data: data, encoding: .utf8) else {
                    return onError("Apple didn't send a sign-in token")
                }
                let raw = nonce.raw
                Task { await action(token, raw) }
            case let .failure(error):
                if (error as? ASAuthorizationError)?.code != .canceled { onError(error.localizedDescription) }
            }
        }
        .signInWithAppleButtonStyle(scheme == .dark ? .white : .black)
        .frame(height: 52)
    }
}
