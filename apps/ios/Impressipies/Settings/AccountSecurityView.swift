import SwiftUI

/// Confirm an email and set a password, so the phone can sign back in without an invite.
struct AccountSecurityView: View {
    @Environment(Session.self) private var session

    @State private var email = ""
    @State private var sentTo: String?
    @State private var code = ""
    @State private var password = ""
    @State private var busy = false
    @State private var error: String?
    @State private var saved: String?

    var body: some View {
        Form {
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
        .navigationTitle("Email & password")
        .navigationBarTitleDisplayMode(.inline)
    }

    private func run(_ work: @escaping () async throws -> Void) {
        error = nil
        saved = nil
        busy = true
        Task {
            defer { busy = false }
            do { try await work() } catch { self.error = APIError.from(error).message }
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
