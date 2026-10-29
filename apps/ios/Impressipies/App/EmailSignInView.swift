import SwiftUI

/// Sign in with a password, or with an emailed link and six-digit code.
struct EmailSignInView: View {
    @Environment(Session.self) private var session
    let onJoin: () -> Void

    private enum Mode { case password, link }

    @State private var mode = Mode.password
    @State private var email = ""
    @State private var password = ""
    @State private var code = ""
    @State private var sent = false
    @State private var serverText = ""
    @State private var scanning = false
    @State private var busy = false
    @State private var error: String?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                Image("Mark")
                    .resizable()
                    .frame(width: 72, height: 72)
                    .rotationEffect(.degrees(-4))
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 6) {
                    Scribble(text: "welcome back")
                    Text("Sign in").font(.display()).foregroundStyle(Palette.ink)
                }

                field("Email") {
                    TextField("you@example.com", text: $email)
                        .textContentType(.username)
                        .keyboardType(.emailAddress)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .disabled(sent)
                }

                if mode == .password {
                    field("Password") {
                        SecureField("Password", text: $password)
                            .textContentType(.password)
                            .submitLabel(.go)
                            .onSubmit(submit)
                    }
                } else if sent {
                    Text("If \(email) belongs to a member, a link and a 6-digit code are on their way. Tap the link on this phone, or enter the code.")
                        .font(.callout)
                        .foregroundStyle(Palette.inkMuted)
                    field("6-digit code") {
                        TextField("000000", text: $code)
                            .textContentType(.oneTimeCode)
                            .keyboardType(.numberPad)
                            .onChange(of: code) { _, v in code = String(v.filter(\.isNumber).prefix(6)) }
                    }
                }

                if let error {
                    Label(error, systemImage: "exclamationmark.triangle.fill")
                        .font(.callout.weight(.semibold))
                        .foregroundStyle(Palette.danger)
                }

                Button(action: submit) {
                    if busy { ProgressView().tint(Palette.onAccent) } else { Text(primaryTitle) }
                }
                .buttonStyle(PopButtonStyle())
                .disabled(!canSubmit)

                VStack(spacing: 12) {
                    if mode == .password {
                        Button("Forgot your password, or new phone? Email me a link") { switchTo(.link) }
                    } else {
                        if sent { Button("Use a different email") { sent = false; code = ""; error = nil } }
                        Button("Sign in with a password instead") { switchTo(.password) }
                    }
                    Button("Have an invite link? Join the family", action: onJoin)
                }
                .font(.footnote.weight(.bold))
                .frame(maxWidth: .infinity)

                serverSection
            }
            .padding(20)
            .padding(.top, 24)
        }
        .scrollDismissesKeyboard(.interactively)
        .pageBackground()
        .onAppear { serverText = session.server.absoluteString }
        .sheet(isPresented: $scanning) {
            QRScannerSheet { url in
                session.setServer(url)
                serverText = url.absoluteString
            }
        }
    }

    private var primaryTitle: String {
        mode == .password ? "Sign in" : (sent ? "Sign in with code" : "Email me a link")
    }

    private var canSubmit: Bool {
        if busy || email.trimmingCharacters(in: .whitespaces).isEmpty { return false }
        return mode == .password ? !password.isEmpty : (!sent || code.count == 6)
    }

    private func field<C: View>(_ title: String, @ViewBuilder _ content: () -> C) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title).font(.subheadline.weight(.bold)).foregroundStyle(Palette.ink)
            content()
                .padding(.horizontal, 14)
                .frame(minHeight: 52)
                .sticker(radius: 16, shadow: 3)
        }
    }

    private var serverSection: some View {
        DisclosureGroup("Server: \(session.server.host() ?? session.server.absoluteString)") {
            VStack(alignment: .leading, spacing: 8) {
                TextField("Server", text: $serverText)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .keyboardType(.URL)
                    .padding(.horizontal, 14)
                    .frame(minHeight: 48)
                    .sticker(radius: 14, shadow: 2)
                HStack {
                    Button("Impressipes") { serverText = APIConfig.defaultServer.absoluteString }
                    Button("Local dev") { serverText = "http://localhost:8080" }
                    Button { scanning = true } label: { Label("Scan QR", systemImage: "qrcode.viewfinder") }
                }
                .font(.footnote.weight(.bold))
            }
            .padding(.top, 8)
        }
        .tint(Palette.ink)
        .font(.footnote.weight(.bold))
    }

    private func switchTo(_ next: Mode) {
        mode = next
        error = nil
        sent = false
        code = ""
    }

    private func submit() {
        guard canSubmit else { return }
        guard let server = URL(string: serverText.trimmingCharacters(in: .whitespaces)), server.scheme != nil else {
            error = "Check the server address."
            return
        }
        if server != session.server { session.setServer(server) }
        error = nil
        busy = true
        let mail = email.trimmingCharacters(in: .whitespaces)
        Task {
            defer { busy = false }
            do {
                if mode == .password {
                    try await session.signIn(email: mail, password: password)
                } else if sent {
                    try await session.verify(email: mail, code: code)
                } else {
                    try await session.requestLink(email: mail)
                    sent = true
                }
            } catch {
                self.error = APIError.from(error).message
            }
        }
    }
}
