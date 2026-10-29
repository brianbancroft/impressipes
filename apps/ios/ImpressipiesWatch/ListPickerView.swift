import SwiftUI

/// K1: the watch always opens here. One row per list with how much is left.
struct ListPickerView: View {
    @Environment(WatchSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase
    @State private var lists: [ShoppingList] = OfflineStore.shared.lists
    @State private var remaining: [String: Int] = Dictionary(
        uniqueKeysWithValues: OfflineStore.shared.lists.compactMap { l in
            OfflineStore.shared.snapshot(l.id).map { (l.id, OfflineStore.shared.overlay($0.detail, listId: l.id).remaining) }
        }
    )
    @State private var loaded = false
    @State private var error: String?

    var body: some View {
        NavigationStack {
            List {
                ForEach(lists, id: \.id) { list in
                    NavigationLink(value: list) {
                        PickerRow(list: list, remaining: remaining[list.id])
                    }
                    .listRowBackground(
                        RoundedRectangle(cornerRadius: 12)
                            .fill(WatchPalette.surface)
                            .overlay(alignment: .leading) { Rectangle().fill(WatchPalette.colour(list.colour)).frame(width: 4) }
                            .clipShape(RoundedRectangle(cornerRadius: 12))
                    )
                    .listRowInsets(EdgeInsets())
                }
                if loaded, lists.isEmpty {
                    Text(error ?? "No lists yet. Make one on your iPhone.")
                        .font(.system(size: 12))
                        .foregroundStyle(WatchPalette.inkMuted)
                        .multilineTextAlignment(.center)
                        .frame(maxWidth: .infinity)
                        .listRowBackground(Color.clear)
                }
            }
            .overlay { if !loaded { ProgressView() } }
            .navigationTitle("Lists")
            .navigationDestination(for: ShoppingList.self) { list in
                ListDetailView(list: list) { remaining[list.id] = $0 }
            }
            .containerBackground(WatchPalette.bg, for: .navigation)
        }
        .task { await load() }
        .onChange(of: scenePhase) { _, phase in
            if phase == .active { Task { await load() } }
        }
    }

    private func load() async {
        _ = await OfflineStore.shared.flush(using: session.api)
        do {
            let fetched = try await session.api.lists().sorted { $0.sortOrder < $1.sortOrder }
            lists = fetched
            OfflineStore.shared.save(lists: fetched)
            error = nil
            // Counts come from each list's items; there are only a handful of lists.
            let api = session.api
            await withTaskGroup(of: (String, ListDetail?).self) { group in
                for l in fetched {
                    group.addTask { (l.id, try? await api.list(l.id)) }
                }
                for await (id, fresh) in group {
                    if let fresh {
                        // Counts include check-offs still waiting to sync.
                        remaining[id] = OfflineStore.shared.overlay(fresh, listId: id).remaining
                    }
                }
            }
        } catch {
            self.error = APIError.from(error).message
        }
        loaded = true
    }
}

private struct PickerRow: View {
    let list: ShoppingList
    let remaining: Int?

    var body: some View {
        HStack(spacing: 8) {
            Text(list.emoji.isEmpty ? "🛒" : list.emoji)
                .font(.system(size: 20))
                .frame(width: 26)
            Text(list.name)
                .font(.display(15))
                .lineLimit(2)
                .frame(maxWidth: .infinity, alignment: .leading)
            if let remaining {
                if remaining == 0 {
                    Label("All done", systemImage: "checkmark")
                        .font(.system(size: 13, weight: .bold, design: .rounded))
                        .foregroundStyle(WatchPalette.success)
                } else {
                    Text("\(remaining)")
                        .font(.system(size: 13, weight: .bold, design: .rounded))
                        .padding(.horizontal, 9)
                        .padding(.vertical, 2)
                        .background(Capsule().fill(WatchPalette.accentSoft))
                }
            }
        }
        .padding(.leading, 10)
        .padding(.trailing, 10)
        .frame(minHeight: 44)
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(list.name), \(remaining.map { $0 == 0 ? "all done" : "\($0) left" } ?? "")")
    }
}
