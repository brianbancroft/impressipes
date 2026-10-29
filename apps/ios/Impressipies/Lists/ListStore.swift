import Foundation
import Observation

/// One list's items, with optimistic edits that roll back if the server says no.
@MainActor @Observable
final class ListStore {
    let listId: String
    private(set) var detail: ListDetail?
    private(set) var error: String?
    /// Recipe titles for the "from …" line on items added from a recipe.
    private(set) var recipeTitles: [String: String] = [:]

    private let api: () -> API

    init(listId: String, api: @escaping () -> API) {
        self.listId = listId
        self.api = api
    }

    var list: ShoppingList? { detail?.list }
    var categories: [Category] { detail?.categories ?? [] }
    var items: [Item] { detail?.items ?? [] }

    func category(_ id: String?) -> Category? {
        guard let id else { return nil }
        return categories.first { $0.id == id }
    }

    func load() async {
        do {
            detail = try await api().list(listId)
            error = nil
            await loadRecipeTitles()
        } catch {
            self.error = APIError.from(error).message
        }
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
        replace(next)
        try? await attempt(item) {
            replace(try await api().updateItem(item.id, .init(checked: next.checked)))
        }
    }

    @discardableResult
    func add(name: String, quantityText: String, note: String? = nil, categoryId: String?) async throws -> Item {
        let created = try await api().createItem(
            listId: listId,
            name: name,
            quantityText: quantityText.isEmpty ? nil : quantityText,
            note: note?.isEmpty == false ? note : nil,
            categoryId: categoryId
        )
        replace(created)
        return created
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
