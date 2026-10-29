import SwiftUI

/// First run: which Impressipes server this phone talks to. There's no built-in one,
/// so scan the code from App setup on the web, or type the address.
struct ServerSetupView: View {
    @Environment(Session.self) private var session
    let onJoin: () -> Void

    @State private var address = ""
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
                    Scribble(text: "first things first")
                    Text("Find your kitchen").font(.display()).foregroundStyle(Palette.ink)
                    Text("Impressipes runs on your family's own server. Open it on the web, choose App setup from your profile menu, and scan the code.")
                        .foregroundStyle(Palette.inkMuted)
                }

                VStack(alignment: .leading, spacing: 16) {
                    Button { scanning = true } label: { Label("Scan QR code", systemImage: "qrcode.viewfinder") }
                        .buttonStyle(PopButtonStyle())
                        .disabled(busy)

                    Text("or type the address")
                        .font(.footnote.weight(.semibold))
                        .foregroundStyle(Palette.inkMuted)
                        .frame(maxWidth: .infinity)

                    VStack(alignment: .leading, spacing: 8) {
                        Text("Server address").font(.subheadline.weight(.bold)).foregroundStyle(Palette.ink)
                        HStack {
                            TextField("impressipes.example.com", text: $address)
                                .textContentType(.URL)
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                                .keyboardType(.URL)
                                .submitLabel(.go)
                                .onSubmit(submit)
                            PasteButton(payloadType: String.self) { strings in
                                if let s = strings.first { address = s }
                            }
                            .labelStyle(.iconOnly)
                            .buttonBorderShape(.capsule)
                        }
                        .padding(.leading, 14)
                        .padding(.trailing, 6)
                        .frame(minHeight: 52)
                        .sticker(radius: 16, shadow: 3)
                        #if DEBUG
                        Button("Use local dev (localhost:8080)") { address = "http://localhost:8080" }
                            .font(.footnote.weight(.bold))
                        #endif
                    }

                    if let error {
                        Label(error, systemImage: "exclamationmark.triangle.fill")
                            .font(.callout.weight(.semibold))
                            .foregroundStyle(Palette.danger)
                    }

                    Button(action: submit) {
                        if busy { ProgressView().tint(Palette.ink) } else { Text("Continue") }
                    }
                    .buttonStyle(PopButtonStyle(fill: Palette.surface, foreground: Palette.ink))
                    .disabled(busy || address.trimmingCharacters(in: .whitespaces).isEmpty)
                }
                .padding(18)
                .sticker(radius: 24)

                Button("Have an invite link? Join the family", action: onJoin)
                    .font(.footnote.weight(.bold))
                    .frame(maxWidth: .infinity)
            }
            .padding(20)
            .padding(.top, 24)
        }
        .scrollDismissesKeyboard(.interactively)
        .pageBackground()
        .sheet(isPresented: $scanning) {
            QRScannerSheet { url in
                address = url.absoluteString
                connect(url)
            }
        }
    }

    private func submit() {
        guard let url = ServerCode.origin(typed: address) else {
            error = "Check the server address."
            return
        }
        connect(url)
    }

    private func connect(_ url: URL) {
        error = nil
        busy = true
        Task {
            defer { busy = false }
            do { try await session.connect(to: url) } catch { self.error = APIError.from(error).message }
        }
    }
}
