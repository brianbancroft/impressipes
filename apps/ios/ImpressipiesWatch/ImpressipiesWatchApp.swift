import SwiftUI

@main
struct ImpressipiesWatchApp: App {
    @State private var session = WatchSession()

    var body: some Scene {
        WindowGroup {
            Group {
                switch session.state {
                case .waiting: WaitingForPhoneView()
                case .signedIn: ListPickerView()
                }
            }
            .environment(session)
            .preferredColorScheme(.dark)
            .tint(WatchPalette.accent)
        }
    }
}

/// Shown until the phone hands over a session.
struct WaitingForPhoneView: View {
    var body: some View {
        ScrollView {
            VStack(spacing: 6) {
                Text("🥧").font(.system(size: 40))
                Text("Sign in on your iPhone")
                    .font(.display(15))
                    .multilineTextAlignment(.center)
                Text("Open Impressipes on your phone and your lists show up here.")
                    .font(.system(size: 12))
                    .foregroundStyle(WatchPalette.inkMuted)
                    .multilineTextAlignment(.center)
            }
            .padding(.horizontal, 10)
        }
        .containerBackground(WatchPalette.bg, for: .navigation)
    }
}
