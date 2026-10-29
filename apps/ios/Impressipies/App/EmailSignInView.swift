import SwiftUI

/// Sign in to the chosen server, as on the web: a passkey or Apple, a password,
/// or an emailed link and six-digit code.
struct EmailSignInView: View {
    @Environment(Session.self) private var session
    let onJoin: () -> Void

    private enum Mode: Hashable { case password, link }

    @State private var mode = Mode.password
    @State private var email = ""
    @State private var password = ""
    @State private var code = ""
    @State private var sent = false
    @State private var busy = false
    @State private var error: String?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                HStack(alignment: .top) {
                    Image("Mark")
                        .resizable()
                        .frame(width: 72, height: 72)
                        .rotationEffect(.degrees(-4))
                        .accessibilityHidden(true)
                    Spacer()
                    serverChip
                }
                VStack(alignment: .leading, spacing: 6) {
                    Scribble(text: "welcome back")
                    Text("Sign in").font(.display()).foregroundStyle(Palette.ink)
                    Text("Use a passkey or Apple, your password, or a one-time link by email.")
                        .foregroundStyle(Palette.inkMuted)
                }

                form
                    .padding(18)
                    .sticker(radius: 24)

                Button("Joining the family? Use your invite link", action: onJoin)
                    .font(.footnote.weight(.bold))
                    .frame(maxWidth: .infinity)
            }
            .padding(20)
            .padding(.top, 24)
        }
        .scrollDismissesKeyboard(.interactively)
        .pageBackground()
        .task { await session.loadAuthConfig() }
    }

    private var form: some View {
        VStack(alignment: .leading, spacing: 16) {
            VStack(spacing: 12) {
                if session.authConfig?.passkeys == true {
                    Button { signInWithPasskey() } label: {
                        Label("Sign in with a passkey", systemImage: "person.badge.key.fill")
                    }
                    .buttonStyle(PopButtonStyle())
                    .disabled(busy)
                }
                AppleAuthButton(label: .signIn, onError: { error = $0 }) { token, nonce in
                    await signInWithApple(token: token, nonce: nonce)
                }
                .disabled(busy)
                Text("or use your email")
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(Palette.inkMuted)
            }
            .frame(maxWidth: .infinity)

            Picker("Sign-in method", selection: Binding(get: { mode }, set: switchTo)) {
                Label("Password", systemImage: "key.fill").tag(Mode.password)
                Label("Email link", systemImage: "envelope.fill").tag(Mode.link)
            }
            .pickerStyle(.segmented)

            field("Email") {
                TextField("you@example.com", text: $email)
                    .textContentType(.username)
                    .keyboardType(.emailAddress)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .disabled(sent)
            }

            switch (mode, sent) {
            case (.password, _):
                field("Password", accessory: Button("Forgot it?") { switchTo(.link) }) {
                    SecureField("Password", text: $password)
                        .textContentType(.password)
                        .submitLabel(.go)
                        .onSubmit(submit)
                }
            case (.link, false):
                Text("New phone or forgot your password? We'll email a link and a 6-digit code.")
                    .font(.callout)
                    .foregroundStyle(Palette.inkMuted)
            case (.link, true):
                (Text("If ") + Text(email).bold().foregroundStyle(Palette.ink)
                 + Text(" belongs to a member, a link and a 6-digit code are on their way. Tap the link on this phone, or enter the code here."))
                    .font(.callout)
                    .foregroundStyle(Palette.inkMuted)
                    .padding(12)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(RoundedRectangle(cornerRadius: 14, style: .continuous).fill(Palette.surface2))
                field("6-digit code") {
                    TextField("000000", text: $code)
                        .textContentType(.oneTimeCode)
                        .keyboardType(.numberPad)
                        .font(.system(.title, design: .monospaced, weight: .bold))
                        .tracking(8)
                        .multilineTextAlignment(.center)
                        .onChange(of: code) { _, v in code = String(v.filter(\.isNumber).prefix(6)) }
                }
            }

            if let error {
                Label(error, systemImage: "exclamationmark.triangle.fill")
                    .font(.callout.weight(.semibold))
                    .foregroundStyle(Palette.danger)
            }

            Button(action: submit) {
                if busy {
                    ProgressView().tint(Palette.onAccent)
                } else if mode == .link && !sent {
                    Label("Email me a link", systemImage: "envelope.fill")
                } else {
                    Text(sent ? "Sign in with code" : "Sign in")
                }
            }
            .buttonStyle(PopButtonStyle())
            .disabled(!canSubmit)

            if sent {
                Button("Use a different email") { sent = false; code = ""; error = nil }
                    .font(.footnote.weight(.bold))
                    .frame(maxWidth: .infinity)
            }
        }
    }

    /// Which server this signs in to; tapping it goes back to pick another.
    private var serverChip: some View {
        Button { session.clearServer() } label: {
            Chip(label: session.server?.host() ?? "", systemImage: "server.rack")
        }
        .buttonStyle(.plain)
        .disabled(busy)
        .accessibilityLabel("Server \(session.server?.host() ?? ""). Change server")
    }

    private var canSubmit: Bool {
        if busy || email.trimmingCharacters(in: .whitespaces).isEmpty { return false }
        return mode == .password ? !password.isEmpty : (!sent || code.count == 6)
    }

    private func field<C: View>(_ title: String, @ViewBuilder _ content: () -> C) -> some View {
        field(title, accessory: EmptyView(), content)
    }

    private func field<A: View, C: View>(_ title: String, accessory: A, @ViewBuilder _ content: () -> C) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(alignment: .firstTextBaseline) {
                Text(title).font(.subheadline.weight(.bold)).foregroundStyle(Palette.ink)
                Spacer()
                accessory.font(.footnote.weight(.bold))
            }
            content()
                .padding(.horizontal, 14)
                .frame(minHeight: 52)
                .sticker(radius: 16, shadow: 3)
        }
    }

    private func switchTo(_ next: Mode) {
        mode = next
        error = nil
        sent = false
        code = ""
    }

    private func signInWithPasskey() {
        error = nil
        busy = true
        Task {
            defer { busy = false }
            do { try await session.signInWithPasskey() } catch { self.error = APIError.from(error).message }
        }
    }

    private func signInWithApple(token: String, nonce: String) async {
        error = nil
        busy = true
        defer { busy = false }
        do { try await session.signInWithApple(identityToken: token, nonce: nonce) } catch { self.error = APIError.from(error).message }
    }

    private func submit() {
        guard canSubmit else { return }
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
