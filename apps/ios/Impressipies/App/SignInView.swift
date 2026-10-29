import SwiftUI

/// Sign in with an invite link from the web app (Settings → Family on the web,
/// or "Invite again" for yourself to add this phone).
struct SignInView: View {
    @Environment(Session.self) private var session

    @State private var serverText = ""
    @State private var link = ""
    @State private var invite: InviteInfo?
    @State private var token = ""
    @State private var name = ""
    @State private var color = Palette.memberColours[0]
    @State private var busy = false
    @State private var error: String?
    @State private var scanning = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                Image("Mark")
                    .resizable()
                    .frame(width: 72, height: 72)
                    .rotationEffect(.degrees(-4))
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 6) {
                    Scribble(text: invite == nil ? "pull up a chair" : "nearly there")
                    Text(invite == nil ? "Join the family" : (invite?.reinvite == true ? "Welcome back" : "Who's this?"))
                        .font(.display())
                        .foregroundStyle(Palette.ink)
                    Text(invite == nil
                         ? "Paste the invite link from Impressipes on the web. Ask whoever runs your family's account for one."
                         : "Pick a name and colour so everyone knows who added what.")
                        .foregroundStyle(Palette.inkMuted)
                }

                if invite == nil { linkStep } else { profileStep }

                if let error {
                    Label(error, systemImage: "exclamationmark.triangle.fill")
                        .font(.callout.weight(.semibold))
                        .foregroundStyle(Palette.danger)
                }
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

    private var linkStep: some View {
        VStack(alignment: .leading, spacing: 16) {
            VStack(alignment: .leading, spacing: 8) {
                Button { scanning = true } label: { Label("Scan QR code", systemImage: "qrcode.viewfinder") }
                    .buttonStyle(PopButtonStyle())
                Text("Server: \(session.server.host() ?? session.server.absoluteString)")
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(Palette.inkMuted)
                Text("Open Impressipes on the web and choose App setup from your profile menu.")
                    .font(.footnote)
                    .foregroundStyle(Palette.inkMuted)
            }
            VStack(alignment: .leading, spacing: 8) {
                Text("Invite link").font(.subheadline.weight(.bold)).foregroundStyle(Palette.ink)
                HStack {
                    TextField("https://…/invite/…", text: $link)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                        .submitLabel(.go)
                        .onSubmit(check)
                    PasteButton(payloadType: String.self) { strings in
                        if let s = strings.first { link = s; check() }
                    }
                    .labelStyle(.iconOnly)
                    .buttonBorderShape(.capsule)
                }
                .padding(.leading, 14)
                .padding(.trailing, 6)
                .frame(minHeight: 52)
                .sticker(radius: 16, shadow: 3)
            }

            DisclosureGroup("Server") {
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
                    }
                    .font(.footnote.weight(.bold))
                }
                .padding(.top, 8)
            }
            .tint(Palette.ink)
            .font(.subheadline.weight(.bold))

            Button(action: check) {
                if busy { ProgressView().tint(Palette.onAccent) } else { Text("Continue") }
            }
            .buttonStyle(PopButtonStyle())
            .disabled(busy || link.trimmingCharacters(in: .whitespaces).isEmpty)
        }
    }

    private var profileStep: some View {
        VStack(alignment: .leading, spacing: 18) {
            HStack(spacing: 14) {
                Text(String(name.prefix(1)).uppercased().ifEmpty("?"))
                    .font(.system(.title, design: .rounded, weight: .heavy))
                    .foregroundStyle(.white)
                    .frame(width: 60, height: 60)
                    .background(Circle().fill(Color(hex: color)))
                    .overlay(Circle().strokeBorder(Palette.outline, lineWidth: 2.5))
                TextField("Your name", text: $name)
                    .font(.title3.weight(.semibold))
                    .padding(.horizontal, 14)
                    .frame(minHeight: 52)
                    .sticker(radius: 16, shadow: 3)
            }
            Text("Colour").font(.subheadline.weight(.bold)).foregroundStyle(Palette.ink)
            HStack(spacing: 10) {
                ForEach(Palette.memberColours, id: \.self) { c in
                    Button { color = c } label: {
                        Circle()
                            .fill(Color(hex: c))
                            .frame(width: 34, height: 34)
                            .overlay(Circle().strokeBorder(Palette.outline, lineWidth: c == color ? 3.5 : 2))
                            .scaleEffect(c == color ? 1.12 : 1)
                    }
                    .accessibilityLabel(c)
                    .accessibilityAddTraits(c == color ? .isSelected : [])
                }
            }
            Button(action: accept) {
                if busy { ProgressView().tint(Palette.onAccent) } else { Text("Let's go") }
            }
            .buttonStyle(PopButtonStyle())
            .disabled(busy || name.trimmingCharacters(in: .whitespaces).isEmpty)
            Button("Use a different link") { invite = nil; error = nil }
                .font(.footnote.weight(.bold))
                .frame(maxWidth: .infinity)
        }
    }

    /// The token is the last path part of …/invite/<token>; a bare token works too.
    private static func token(from text: String) -> String? {
        let t = text.trimmingCharacters(in: .whitespacesAndNewlines)
        if let url = URL(string: t), let i = url.pathComponents.firstIndex(of: "invite"), i + 1 < url.pathComponents.count {
            return url.pathComponents[i + 1]
        }
        return t.isEmpty || t.contains("/") ? nil : t
    }

    private func check() {
        error = nil
        guard let tok = Self.token(from: link) else {
            error = "That doesn't look like an invite link."
            return
        }
        guard let server = URL(string: serverText.trimmingCharacters(in: .whitespaces)), server.scheme != nil else {
            error = "Check the server address."
            return
        }
        // An invite link carries its own server: use that origin.
        if let url = URL(string: link.trimmingCharacters(in: .whitespacesAndNewlines)), let host = url.host(), let scheme = url.scheme {
            var c = URLComponents()
            c.scheme = scheme
            c.host = host
            c.port = url.port
            // The web dev server on :4200 proxies to the API on :8080.
            if host == "localhost", url.port == 4200 { c.port = 8080 }
            session.setServer(c.url ?? server)
            serverText = session.server.absoluteString
        } else {
            session.setServer(server)
        }
        busy = true
        Task {
            defer { busy = false }
            do {
                let info = try await session.api.invite(token: tok)
                switch info.state {
                case .valid:
                    token = tok
                    name = info.name ?? ""
                    color = info.color ?? color
                    invite = info
                case .used: error = "This invite was already used. Ask for a new link."
                case .expired: error = "This invite has expired. Ask for a new link."
                case .disabled: error = "This account is disabled."
                case .invalid: error = "We can't find that invite."
                }
            } catch {
                self.error = APIError.from(error).message
            }
        }
    }

    private func accept() {
        busy = true
        error = nil
        Task {
            defer { busy = false }
            do {
                try await session.acceptInvite(token: token, name: name.trimmingCharacters(in: .whitespaces), color: color)
            } catch {
                self.error = APIError.from(error).message
            }
        }
    }
}

extension String {
    func ifEmpty(_ fallback: String) -> String { isEmpty ? fallback : self }
}
