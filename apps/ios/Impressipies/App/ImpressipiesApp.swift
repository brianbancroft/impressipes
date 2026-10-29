import SwiftUI

@main
struct ImpressipiesApp: App {
    @State private var session = Session()
    @AppStorage("appearance") private var appearance = Appearance.system

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(session)
                .preferredColorScheme(appearance.colorScheme)
                .tint(Palette.accent)
        }
    }
}

enum Appearance: String, CaseIterable, Identifiable {
    case system, light, dark
    var id: String { rawValue }
    var label: String { rawValue.capitalized }
    var colorScheme: ColorScheme? {
        switch self {
        case .system: nil
        case .light: .light
        case .dark: .dark
        }
    }
}

struct RootView: View {
    @Environment(Session.self) private var session
    @Environment(\.scenePhase) private var scenePhase

    var body: some View {
        Group {
            switch session.state {
            case .loading:
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity).pageBackground()
            case .signedOut:
                SignInView()
            case .signedIn:
                MainTabs()
            }
        }
        .task { await session.restore() }
        .onChange(of: scenePhase) { _, phase in
            // Hold the event stream only while the app is on screen.
            if phase == .active { session.startEvents() } else if phase == .background { session.stopEvents() }
        }
    }
}

struct MainTabs: View {
    enum Tab: Hashable { case lists, recipes, settings }
    @State private var tab = Tab.lists

    var body: some View {
        TabView(selection: $tab) {
            ListsView()
                .tabItem { Label("Lists", systemImage: "checklist") }
                .tag(Tab.lists)
            RecipesView(openLists: { tab = .lists })
                .tabItem { Label("Recipes", systemImage: "fork.knife") }
                .tag(Tab.recipes)
            SettingsView()
                .tabItem { Label("Settings", systemImage: "gearshape") }
                .tag(Tab.settings)
        }
    }
}
