import SwiftUI

struct SettingsView: View {
    @Environment(Session.self) private var session
    @AppStorage("appearance") private var appearance = Appearance.system
    @State private var confirmSignOut = false
    @State private var scanning = false
    @State private var scanned: URL?

    var body: some View {
        NavigationStack {
            Form {
                if let m = session.member {
                    Section("Account") {
                        HStack(spacing: 14) {
                            Text(String(m.name.prefix(1)).uppercased())
                                .font(.system(.title2, design: .rounded, weight: .heavy))
                                .foregroundStyle(.white)
                                .frame(width: 48, height: 48)
                                .background(Circle().fill(Color(hex: m.color)))
                                .overlay(Circle().strokeBorder(Palette.outline, lineWidth: 2))
                            VStack(alignment: .leading) {
                                Text(m.name).font(.headline)
                                Text(m.role == .admin ? "Admin" : "Family member")
                                    .font(.subheadline)
                                    .foregroundStyle(Palette.inkMuted)
                            }
                        }
                        .padding(.vertical, 4)
                        NavigationLink("Sign-in & security") { AccountSecurityView() }
                    }
                }

                Section("Appearance") {
                    Picker("Theme", selection: $appearance) {
                        ForEach(Appearance.allCases) { Text($0.label).tag($0) }
                    }
                    .pickerStyle(.segmented)
                }

                Section {
                    if let server = session.server {
                        LabeledContent("Server", value: server.host() ?? server.absoluteString)
                        Link("Open Impressipes on the web", destination: server)
                    }
                    Button { scanning = true } label: { Label("Change server with QR code", systemImage: "qrcode.viewfinder") }
                } header: {
                    Text("Sync")
                } footer: {
                    Text("Recipes are added and edited on the web. Family members are managed there too.")
                }

                Section {
                    Button("Sign out", role: .destructive) { confirmSignOut = true }
                }
            }
            .scrollContentBackground(.hidden)
            .pageBackground()
            .navigationTitle("Settings")
            .sheet(isPresented: $scanning) { QRScannerSheet { scanned = $0 } }
            .confirmationDialog("Switch to \(scanned?.host() ?? "this server")?", isPresented: Binding(get: { scanned != nil }, set: { if !$0 { scanned = nil } }), titleVisibility: .visible) {
                Button("Switch and sign out", role: .destructive) {
                    if let url = scanned { session.signOut(); session.setServer(url) }
                    scanned = nil
                }
            } message: {
                Text("You'll be signed out and need an invite link from that server.")
            }
            .confirmationDialog("Sign out of Impressipes?", isPresented: $confirmSignOut, titleVisibility: .visible) {
                Button("Sign out", role: .destructive) { session.signOut() }
            } message: {
                Text("To sign back in you'll need your email and password, an emailed link, or a new invite link.")
            }
        }
    }
}
