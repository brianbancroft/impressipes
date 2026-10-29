import SwiftUI
import WatchKit

/// K2: unchecked items under coloured aisle headers, then a collapsed
/// "Got it (n)". Tap a row to cross it off; open Got it and tap it again to
/// put it back on the list.
struct ListDetailView: View {
    let list: ShoppingList
    /// Tells the picker the new unchecked count.
    let onRemaining: (Int) -> Void

    @Environment(WatchSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var store: WatchListStore?
    /// Rows showing their check before they leave for Got it.
    @State private var leaving: Set<String> = []
    @State private var gotOpen = false
    @State private var lastHaptic = Date.distantPast

    var body: some View {
        List {
            if let store, store.detail != nil {
                content(store)
            } else if let error = store?.error {
                message("Couldn't load", error)
            }
        }
        .overlay { if store?.detail == nil, store?.error == nil { ProgressView() } }
        .navigationTitle("\(list.emoji) \(list.name)")
        .containerBackground(WatchPalette.bg, for: .navigation)
        .task {
            let store = WatchListStore(listId: list.id, api: { session.api })
            self.store = store
            // No event stream on the watch: refresh while the list is open.
            while !Task.isCancelled {
                await store.load()
                try? await Task.sleep(for: .seconds(30))
            }
        }
        .onChange(of: scenePhase) { _, phase in
            if phase == .active, let store { Task { await store.load() } }
        }
        .onChange(of: store?.remaining) { _, n in
            if let n { onRemaining(n) }
        }
    }

    @ViewBuilder
    private func content(_ store: WatchListStore) -> some View {
        let open = store.items.filter { !$0.checked || leaving.contains($0.id) }
        let done = store.items.filter { $0.checked && !leaving.contains($0.id) }.sorted { $0.updatedAt > $1.updatedAt }

        if store.isOffline {
            Label("Offline", systemImage: "iphone.slash")
                .font(.system(size: 12, weight: .semibold))
                .foregroundStyle(WatchPalette.inkMuted)
                .frame(maxWidth: .infinity)
                .listRowBackground(Color.clear)
        }

        if open.isEmpty {
            message(done.isEmpty ? "Nothing to get" : "All done", done.isEmpty ? "Add items on your iPhone." : "Everything's in the basket.")
        }

        ForEach(store.groups(open)) { group in
            AisleHeader(category: group.category, count: group.items.filter { !leaving.contains($0.id) }.count)
                .listRowBackground(Color.clear)
                .listRowInsets(EdgeInsets(top: 4, leading: 0, bottom: 0, trailing: 0))
            ForEach(group.items, id: \.id) { item in
                ItemRow(item: item, tint: WatchPalette.colour(group.category?.colour), done: leaving.contains(item.id), pending: store.pendingIds.contains(item.id)) {
                    check(item, in: store)
                }
            }
        }

        if !done.isEmpty {
            Button {
                gotOpen.toggle()
            } label: {
                HStack(spacing: 6) {
                    Image(systemName: gotOpen ? "chevron.down" : "chevron.right")
                        .font(.system(size: 12, weight: .bold))
                    Text("Got it (\(done.count))")
                        .font(.system(size: 15, weight: .semibold))
                }
                .foregroundStyle(WatchPalette.inkMuted)
                .frame(maxWidth: .infinity, minHeight: 44, alignment: .leading)
                .contentShape(.rect)
            }
            .buttonStyle(.plain)
            .listRowBackground(Color.clear)
            .accessibilityHint(gotOpen ? "Hides items you've got" : "Shows items you've got")

            if gotOpen {
                ForEach(done, id: \.id) { item in
                    ItemRow(item: item, tint: WatchPalette.colour(store.category(item.categoryId)?.colour), done: true, pending: store.pendingIds.contains(item.id)) {
                        uncheck(item, in: store)
                    }
                }
            }
        }

        if let updated = store.updated {
            Text("Updated \(updated.formatted(date: .omitted, time: .shortened))")
                .font(.system(size: 12))
                .foregroundStyle(WatchPalette.inkMuted)
                .frame(maxWidth: .infinity)
                .listRowBackground(Color.clear)
        }
    }

    private func message(_ title: String, _ body: String) -> some View {
        VStack(spacing: 2) {
            Text("🥧").font(.system(size: 28))
            Text(title).font(.display(15))
            Text(body)
                .font(.system(size: 12))
                .foregroundStyle(WatchPalette.inkMuted)
        }
        .multilineTextAlignment(.center)
        .frame(maxWidth: .infinity)
        .listRowBackground(Color.clear)
    }

    // MARK: Check-off (IMP-18 W1)

    private func check(_ item: Item, in store: WatchListStore) {
        guard !leaving.contains(item.id) else { return }
        haptic(.success)
        if reduceMotion {
            Task { await store.setChecked(item, true, animation: nil) }
            return
        }
        // Ring fills and the name strikes through, the row holds, then leaves.
        withAnimation(.easeOut(duration: 0.12)) { _ = leaving.insert(item.id) }
        Task {
            try? await Task.sleep(for: .milliseconds(320))
            withAnimation(.easeInOut(duration: 0.22)) { _ = leaving.remove(item.id) }
            await store.setChecked(item, true, animation: .easeInOut(duration: 0.22))
        }
    }

    private func uncheck(_ item: Item, in store: WatchListStore) {
        haptic(.click)
        Task { await store.setChecked(item, false, animation: reduceMotion ? nil : .easeInOut(duration: 0.22)) }
    }

    /// At least 250 ms between plays.
    private func haptic(_ type: WKHapticType) {
        let now = Date()
        guard now.timeIntervalSince(lastHaptic) >= 0.25 else { return }
        lastHaptic = now
        WKInterfaceDevice.current().play(type)
    }
}

// MARK: Rows

/// Coloured aisle header: icon and name always, never colour alone.
private struct AisleHeader: View {
    let category: Category?
    let count: Int

    var body: some View {
        let tint = WatchPalette.colour(category?.colour)
        HStack(spacing: 6) {
            Text(category.map { $0.icon.isEmpty ? "🏷️" : $0.icon } ?? "📦")
                .font(.system(size: 9))
                .frame(width: 16, height: 16)
                .background(Circle().fill(tint))
            Text((category?.name ?? "Other").uppercased())
                .font(.display(12))
                .tracking(0.7)
                .lineLimit(1)
                .frame(maxWidth: .infinity, alignment: .leading)
            Text("\(count)")
                .font(.system(size: 12, weight: .bold, design: .rounded))
                .foregroundStyle(WatchPalette.inkMuted)
                .contentTransition(.numericText())
        }
        .padding(.leading, 12)
        .padding(.trailing, 10)
        .frame(height: 28)
        .background {
            RoundedRectangle(cornerRadius: 8)
                .fill(WatchPalette.surface)
                .overlay(RoundedRectangle(cornerRadius: 8).fill(tint.opacity(0.16)))
                .overlay(alignment: .leading) { Rectangle().fill(tint).frame(width: 4) }
                .clipShape(RoundedRectangle(cornerRadius: 8))
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(category?.name ?? "Other"), \(count) left")
        .accessibilityAddTraits(.isHeader)
    }
}

/// The whole row is the tap target; the ring is only an indicator.
private struct ItemRow: View {
    let item: Item
    let tint: Color
    let done: Bool
    var pending = false
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 8) {
                CheckRing(tint: tint, on: done)
                VStack(alignment: .leading, spacing: 0) {
                    Text(item.name)
                        .font(.system(size: 15, weight: .medium))
                        .lineLimit(2)
                        .strikethrough(done)
                        .foregroundStyle(done ? WatchPalette.inkMuted : WatchPalette.ink)
                    if !item.quantityText.isEmpty {
                        Text(item.quantityText)
                            .font(.system(size: 12, weight: .medium))
                            .foregroundStyle(WatchPalette.inkMuted)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                if pending {
                    // Not yet acknowledged by the server; gone on sync.
                    Image(systemName: "clock")
                        .font(.system(size: 12, weight: .semibold))
                        .foregroundStyle(WatchPalette.inkMuted)
                        .accessibilityHidden(true)
                }
            }
            .padding(.leading, 8)
            .padding(.trailing, 10)
            .padding(.vertical, 4)
            .frame(minHeight: 44)
            .contentShape(.rect)
        }
        .buttonStyle(RowPress(tint: tint))
        .listRowBackground(Color.clear)
        .listRowInsets(EdgeInsets(top: 2, leading: 0, bottom: 2, trailing: 0))
        .accessibilityLabel([item.name, item.quantityText].filter { !$0.isEmpty }.joined(separator: ", "))
        .accessibilityValue([done ? "Got it" : "", pending ? "Waiting to sync" : ""].filter { !$0.isEmpty }.joined(separator: ", "))
        .accessibilityHint(done ? "Puts it back on the list" : "Crosses it off")
    }
}

private struct RowPress: ButtonStyle {
    let tint: Color

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .background {
                RoundedRectangle(cornerRadius: 12)
                    .fill(WatchPalette.surface)
                    .overlay(RoundedRectangle(cornerRadius: 12).fill(tint.opacity(configuration.isPressed ? 0.12 : 0)))
            }
            .scaleEffect(configuration.isPressed ? 0.98 : 1)
            .animation(.easeOut(duration: 0.12), value: configuration.isPressed)
    }
}

/// 22 pt ring; done fills it with the aisle colour and draws the check in bg,
/// which keeps its contrast on every dark fill (IMP-18).
private struct CheckRing: View {
    let tint: Color
    let on: Bool

    var body: some View {
        ZStack {
            Circle().strokeBorder(tint, lineWidth: 2.5)
            Circle().fill(tint).opacity(on ? 1 : 0)
            Image(systemName: "checkmark")
                .font(.system(size: 11, weight: .black))
                .foregroundStyle(WatchPalette.bg)
                .opacity(on ? 1 : 0)
                .scaleEffect(on ? 1 : 0.5)
        }
        .frame(width: 22, height: 22)
    }
}

// MARK: Store

/// One list's items. A check applies at once and joins the offline queue; it
/// is sent whenever the server can be reached and never rolled back for being
/// out of range (IMP-58).
@MainActor @Observable
final class WatchListStore {
    /// No successful sync for this long and the title says "Offline".
    static let offlineAfter: TimeInterval = 10 * 60

    let listId: String
    private(set) var detail: ListDetail?
    /// Only set when there is nothing to show at all.
    private(set) var error: String?
    private(set) var updated: Date?
    private(set) var pendingIds: Set<String> = []
    /// Advances with every refresh attempt so the Offline label can appear
    /// while the server stays unreachable.
    private(set) var checkedAt = Date()

    private let api: () -> API
    private let offline: OfflineStore

    init(listId: String, offline: OfflineStore = .shared, api: @escaping () -> API) {
        self.listId = listId
        self.api = api
        self.offline = offline
        if let snap = offline.snapshot(listId) {
            detail = offline.overlay(snap.detail, listId: listId)
            updated = snap.syncedAt
        }
        pendingIds = offline.pendingIds(in: listId)
    }

    var categories: [Category] { detail?.categories ?? [] }
    var items: [Item] { detail?.items ?? [] }
    var remaining: Int? { detail?.remaining }

    var isOffline: Bool {
        guard let updated else { return detail != nil }
        return checkedAt.timeIntervalSince(updated) > Self.offlineAfter
    }

    func category(_ id: String?) -> Category? {
        guard let id else { return nil }
        return categories.first { $0.id == id }
    }

    func load() async {
        await flush()
        checkedAt = .now
        do {
            let fresh = try await api().list(listId)
            let now = Date.now
            offline.save(snapshot: ListSnapshot(detail: fresh, syncedAt: now), for: listId)
            detail = offline.overlay(fresh, listId: listId)
            updated = now
            error = nil
        } catch {
            // Being out of reach is normal: keep what's on screen.
            if detail == nil { self.error = APIError.from(error).message }
        }
    }

    func flush() async {
        for item in await offline.flush(using: api()) where item.listId == listId { replace(item) }
        pendingIds = offline.pendingIds(in: listId)
    }

    struct AisleGroup: Identifiable {
        let id: String
        let category: Category?
        let items: [Item]
    }

    /// Items by aisle in the list's category order, uncategorised last.
    func groups(_ items: [Item]) -> [AisleGroup] {
        let byCat = Dictionary(grouping: items) { category($0.categoryId)?.id ?? "" }
        var out = categories.compactMap { c in
            byCat[c.id].map { AisleGroup(id: c.id, category: c, items: $0.sorted { $0.position < $1.position }) }
        }
        if let rest = byCat[""] {
            out.append(AisleGroup(id: "other", category: nil, items: rest.sorted { $0.position < $1.position }))
        }
        return out
    }

    func setChecked(_ item: Item, _ checked: Bool, animation: Animation?) async {
        var next = item
        next.checked = checked
        next.updatedAt = .now
        offline.enqueue(PendingOp(listId: listId, itemId: item.id, kind: .check(checked), at: next.updatedAt))
        pendingIds = offline.pendingIds(in: listId)
        withAnimation(animation) { replace(next) }
        await flush()
    }

    private func replace(_ item: Item) {
        guard var d = detail, let i = d.value2.items.firstIndex(where: { $0.id == item.id }) else { return }
        d.value2.items[i] = item
        detail = d
    }
}
