import SwiftUI

// Placeholder until the watch list screens (IMP-20) are built.
struct WatchContentView: View {
    var body: some View {
        VStack(spacing: 8) {
            Text("🥧").font(.largeTitle)
            Text("Impressipes").font(.headline)
            Text("Lists are coming to the watch soon.")
                .font(.footnote)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
        }
        .padding()
    }
}
