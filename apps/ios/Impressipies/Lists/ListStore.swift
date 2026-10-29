import Foundation
import Observation

/// One list's items, with optimistic edits that roll back if the server says no.
/// Ticking and adding keep working out of reach of the server: they join the
/// offline queue and go out when it can be reached again (IMP-58).
@MainActor @Observable
final class ListStore {
    let listId: String
    private(set) var detail: ListDetail?
    /// Only set when there is nothing to show at all, or an edit the server refused.
    private(set) var error: String?
    private(set) var updated: Date?
    private(set) var pendingIds: Set<String> = []
    /// Advances with every refresh attempt so "Offline" can appear while the server stays unreachable.
    private(set) var checkedAt = Date()
    /// Recipe titles for the "from …" line on items added from a recipe.
    private(set) var recipeTitles: [String: String] = [:]

    private let api: () -> API
    private let offline: OfflineStore

    /// No successful sync for this long and the list says "Offline".
    static let offlineAfter: TimeInterval = 10 * 60

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

    var isOffline: Bool {
        guard let updated else { return detail != nil }
        return checkedAt.timeIntervalSince(updated) > Self.offlineAfter
    }

    var list: ShoppingList? { detail?.list }
    var categories: [Category] { detail?.categories ?? [] }
    var items: [Item] { detail?.items ?? [] }

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
            await loadRecipeTitles()
        } catch {
            // Being out of reach is normal: keep what's on screen.
            if detail == nil { self.error = APIError.from(error).message }
        }
    }

    /// Sends anything waiting in the queue.
    func flush() async {
        for item in await offline.flush(using: api()) where item.listId == listId { replace(item) }
        pendingIds = offline.pendingIds(in: listId)
    }

    /// Sends the change now; if the server can't be reached, queues it instead.
    /// A server refusal still throws.
    private func send(_ op: PendingOp, _ work: () async throws -> Item) async throws {
        await flush()
        if offline.pending.isEmpty {
            do {
                replace(try await work())
                return
            } catch {
                guard APIError.isUnreachable(error) else { throw APIError.from(error) }
            }
        }
        // Out of reach, or earlier changes are still stuck: keep this one behind them.
        offline.enqueue(op)
        pendingIds = offline.pendingIds(in: listId)
        if let local = offline.localItem(for: op, after: items.map(\.position).max() ?? 0) { replace(local) }
    }

    private func loadRecipeTitles() async {
        let wanted = Set(items.compactMap(\.sourceRecipeId)).subtracting(recipeTitles.keys)
        guard !wanted.isEmpty, let all = try? await api().recipes(query: nil, tag: nil, mark: nil) else { return }
        for r in all { recipeTitles[r.id] = r.title }
    }

    // MARK: Edits

    private func replace(_ item: Item) {
        guard var d = detail else { return }
        if let i = d.value2.items.firstIndex(where: { $0.id == item.id }) {
            d.value2.items[i] = item
        } else {
            d.value2.items.append(item)
        }
        detail = d
    }

    private func remove(_ id: String) {
        detail?.value2.items.removeAll { $0.id == id }
    }

    private func attempt(_ undo: Item?, _ work: () async throws -> Void) async throws {
        do {
            try await work()
        } catch {
            if let undo { replace(undo) }
            self.error = APIError.from(error).message
            throw error
        }
    }

    func toggle(_ item: Item) async {
        var next = item
        next.checked.toggle()
        next.updatedAt = .now
        replace(next)
        try? await attempt(item) {
            try await send(PendingOp(listId: listId, itemId: item.id, kind: .check(next.checked), at: next.updatedAt)) {
                try await api().updateItem(item.id, .init(checked: next.checked))
            }
        }
    }

    @discardableResult
    func add(name: String, quantityText: String, note: String? = nil, categoryId: String?) async throws -> Item {
        let id = UUID().uuidString.lowercased()
        let note = note ?? ""
        let op = PendingOp(listId: listId, itemId: id, kind: .add(name: name, quantityText: quantityText, note: note, categoryId: categoryId), at: .now)
        try await send(op) {
            try await api().createItem(
                listId: listId, id: id, name: name,
                quantityText: quantityText.isEmpty ? nil : quantityText,
                note: note.isEmpty ? nil : note, categoryId: categoryId
            )
        }
        // Either the server's copy, or the local one waiting in the queue.
        return items.first { $0.id == id }!
    }

    func update(_ item: Item, name: String, quantityText: String, note: String, categoryId: String?) async throws {
        var next = item
        next.name = name
        next.quantityText = quantityText
        next.note = note
        next.categoryId = categoryId
        replace(next)
        try await attempt(item) {
            // An empty categoryId clears the category on the server.
            replace(try await api().updateItem(item.id, .init(name: name, quantityText: quantityText, note: note, categoryId: categoryId ?? "")))
        }
    }

    /// Deletes the item; keep the value to offer an undo with restore(_:).
    func delete(_ item: Item) async {
        remove(item.id)
        try? await attempt(item) { try await api().deleteItem(item.id) }
    }

    /// Re-adds a deleted item as a new one with the same details.
    func restore(_ item: Item) async {
        guard let back = try? await add(name: item.name, quantityText: item.quantityText, note: item.note, categoryId: item.categoryId) else { return }
        if item.checked { await toggle(back) }
    }

    func clearChecked() async -> [Item] {
        let done = items.filter(\.checked)
        for item in done { remove(item.id) }
        let api = api()
        await withTaskGroup(of: Void.self) { group in
            for item in done { group.addTask { try? await api.deleteItem(item.id) } }
        }
        return done
    }

    func suggestions(for text: String) async -> [Suggestion] {
        (try? await api().suggestions(listId: listId, query: text)) ?? []
    }

    func dismissError() { error = nil }
}
