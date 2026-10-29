import SwiftUI

/// Passkeys, Apple, email and password: every way this member can sign back in without an invite.
struct AccountSecurityView: View {
    @Environment(Session.self) private var session

    @State private var email = ""
    @State private var sentTo: String?
    @State private var code = ""
    @State private var password = ""
    @State private var busy = false
    @State private var error: String?
    @State private var saved: String?
    @State private var methods: SignInMethods?

    var body: some View {
        Form {
            Section {
                ForEach(methods?.passkeys ?? [], id: \.id) { key in
                    HStack {
                        Image(systemName: "person.badge.key.fill").foregroundStyle(Palette.inkMuted)
                        VStack(alignment: .leading) {
                            Text(key.name)
                            Text("Added \(key.createdAt.formatted(date: .abbreviated, time: .omitted))\(key.synced ? " · syncs with iCloud" : "")")
                                .font(.caption)
                                .foregroundStyle(Palette.inkMuted)
                        }
                        Spacer()
                        Button(role: .destructive) { removePasskey(key) } label: { Image(systemName: "trash") }
                            .buttonStyle(.borderless)
                            .accessibilityLabel("Remove passkey \(key.name)")
                    }
                }
                Button("Add a passkey", action: addPasskey).disabled(busy)
                if let apple = methods?.apple {
                    LabeledContent("Apple ID", value: apple.email ?? "Connected")
                    Button("Disconnect Apple ID", role: .destructive, action: unlinkApple).disabled(busy)
                } else if methods != nil {
                    AppleAuthButton(label: .continue, onError: { error = $0 }) { token, nonce in
                        run {
                            try await session.api.linkApple(identityToken: token, nonce: nonce)
                            saved = "Apple ID connected"
                        }
                    }
                }
            } header: {
                Text("Passkeys & Apple")
            } footer: {
                Text("A passkey saved to iCloud Keychain signs you in with Face ID on all your Apple devices, and on the web.")
            }

            Section {
                if let current = session.member?.email {
                    LabeledContent("Email", value: current)
                } else {
                    Text("No confirmed email yet.").foregroundStyle(Palette.inkMuted)
                }
                if let sentTo {
                    Text("We sent a link and a 6-digit code to \(sentTo). Tap the link on this phone, or enter the code.")
                        .font(.footnote)
                    TextField("6-digit code", text: $code)
                        .textContentType(.oneTimeCode)
                        .keyboardType(.numberPad)
                        .onChange(of: code) { _, v in code = String(v.filter(\.isNumber).prefix(6)) }
                    Button("Confirm email", action: confirm).disabled(busy || code.count != 6)
                    Button("Use a different email") { self.sentTo = nil; code = "" }
                } else {
                    TextField(session.member?.email == nil ? "Email" : "Change email", text: $email)
                        .textContentType(.emailAddress)
                        .keyboardType(.emailAddress)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Button("Send confirmation", action: send)
                        .disabled(busy || email.trimmingCharacters(in: .whitespaces).isEmpty)
                }
            } header: {
                Text("Email")
            } footer: {
                Text("Once confirmed, you can sign in with an emailed link.")
            }

            Section {
                SecureField("New password", text: $password).textContentType(.newPassword)
                Button("Save password", action: savePassword).disabled(busy || password.count < 10)
            } header: {
                Text("Password")
            } footer: {
                Text(session.member?.hasPassword == true ? "You have a password. Saving replaces it. At least 10 characters." : "No password yet. At least 10 characters.")
            }

            if let error {
                Section { Label(error, systemImage: "exclamationmark.triangle.fill").foregroundStyle(Palette.danger) }
            }
            if let saved {
                Section { Label(saved, systemImage: "checkmark.circle.fill").foregroundStyle(Palette.accent) }
            }
        }
        .scrollContentBackground(.hidden)
        .pageBackground()
        .navigationTitle("Sign-in & security")
        .navigationBarTitleDisplayMode(.inline)
        .task { await reload() }
    }

    private func reload() async {
        methods = try? await session.api.signInMethods()
    }

    private func addPasskey() {
        run {
            if try await session.addPasskey() { saved = "Passkey added" }
        }
    }

    private func removePasskey(_ key: Passkey) {
        run {
            try await session.api.deletePasskey(key.id)
            saved = "Passkey removed"
        }
    }

    private func unlinkApple() {
        run {
            try await session.api.unlinkApple()
            saved = "Apple ID disconnected"
        }
    }

    private func run(_ work: @escaping () async throws -> Void) {
        error = nil
        saved = nil
        busy = true
        Task {
            defer { busy = false }
            do {
                try await work()
                await reload()
            } catch { self.error = APIError.from(error).message }
        }
    }

    private func send() {
        let mail = email.trimmingCharacters(in: .whitespaces)
        run {
            try await session.setEmail(mail)
            sentTo = mail.lowercased()
        }
    }

    private func confirm() {
        guard let sentTo else { return }
        run {
            try await session.verify(email: sentTo, code: code)
            self.sentTo = nil
            code = ""
            email = ""
            saved = "Email confirmed"
        }
    }

    private func savePassword() {
        run {
            try await session.setPassword(password)
            password = ""
            saved = "Password saved"
        }
    }
}
