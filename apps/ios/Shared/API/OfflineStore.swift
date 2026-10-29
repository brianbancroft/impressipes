import Foundation

/// A change the server hasn't acknowledged yet.
struct PendingOp: Codable, Equatable {
    enum Kind: Codable, Equatable {
        case check(Bool)
        case add(name: String, quantityText: String, note: String, categoryId: String?)
    }

    let listId: String
    let itemId: String
    let kind: Kind
    let at: Date
}

/// A list as last seen on the server.
struct ListSnapshot: Codable {
    let detail: ListDetail
    let syncedAt: Date
}

/// What the apps keep on disk so they work when the server is out of reach
/// (IMP-58): the last list contents, and changes waiting to be sent. Writes are
/// always accepted here first. Only ticking items and adding them queue; edits,
/// deletes and clearing the basket still need the server.
@MainActor
final class OfflineStore {
    static let shared = OfflineStore()

    private(set) var pending: [PendingOp]
    private(set) var lists: [ShoppingList]
    private var snapshots: [String: ListSnapshot]

    private struct Disk: Codable {
        var pending: [PendingOp] = []
        var lists: [ShoppingList] = []
        var snapshots: [String: ListSnapshot] = [:]
    }

    private let url: URL

    init(url: URL = OfflineStore.defaultURL) {
        self.url = url
        let disk = (try? Data(contentsOf: url)).flatMap { try? Self.decoder.decode(Disk.self, from: $0) } ?? Disk()
        pending = disk.pending
        lists = disk.lists
        snapshots = disk.snapshots
    }

    nonisolated private static var defaultURL: URL {
        let dir = (try? FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true))
            ?? FileManager.default.temporaryDirectory
        return dir.appending(path: "offline.json")
    }

    private static let encoder: JSONEncoder = {
        let e = JSONEncoder()
        e.dateEncodingStrategy = .iso8601
        return e
    }()

    private static let decoder: JSONDecoder = {
        let d = JSONDecoder()
        d.dateDecodingStrategy = .iso8601
        return d
    }()

    func snapshot(_ listId: String) -> ListSnapshot? { snapshots[listId] }

    func save(snapshot: ListSnapshot, for listId: String) {
        snapshots[listId] = snapshot
        persist()
    }

    func save(lists: [ShoppingList]) {
        self.lists = lists
        persist()
    }

    /// Forgets everything; the next person to sign in shouldn't see it.
    func reset() {
        pending = []
        lists = []
        snapshots = [:]
        persist()
    }

    /// Latest tap wins: ticking an item and putting it back before it syncs
    /// leaves one check, not two.
    func enqueue(_ op: PendingOp) {
        if case .check = op.kind {
            pending.removeAll { $0.itemId == op.itemId && { if case .check = $0.kind { true } else { false } }($0) }
        }
        pending.append(op)
        persist()
    }

    func acknowledge(_ op: PendingOp) {
        // A newer tap on the same item stays queued.
        pending.removeAll { $0 == op }
        persist()
    }

    /// Sends queued changes, oldest first, and stops at the first one that
    /// can't get through. Returns the server's version of each item it sent.
    func flush(using api: API) async -> [Item] {
        var sent: [Item] = []
        for op in pending {
            do {
                let item: Item
                switch op.kind {
                case let .check(checked):
                    item = try await api.updateItem(op.itemId, .init(checked: checked))
                case let .add(name, quantityText, note, categoryId):
                    item = try await api.createItem(
                        listId: op.listId, id: op.itemId, name: name,
                        quantityText: quantityText.isEmpty ? nil : quantityText,
                        note: note.isEmpty ? nil : note, categoryId: categoryId
                    )
                }
                acknowledge(op)
                // A newer tap on the same item has replaced this one in the queue.
                if !pendingIds(in: op.listId).contains(item.id) { sent.append(item) }
            } catch {
                if APIError.isUnreachable(error) { break }
                // The server turned it down (item gone, say): drop it and let the next refresh correct the row.
                acknowledge(op)
            }
        }
        return sent
    }

    func pendingIds(in listId: String) -> Set<String> {
        Set(pending.filter { $0.listId == listId }.map(\.itemId))
    }

    /// Puts unacknowledged changes back on top of what the server sent, so a
    /// refresh doesn't undo a tap or an add that hasn't been delivered yet.
    func overlay(_ detail: ListDetail, listId: String) -> ListDetail {
        var detail = detail
        for p in pending where p.listId == listId {
            let index = detail.value2.items.firstIndex { $0.id == p.itemId }
            switch p.kind {
            case let .check(checked):
                if let index {
                    detail.value2.items[index].checked = checked
                    detail.value2.items[index].updatedAt = p.at
                }
            case .add:
                if index == nil, let item = localItem(for: p, after: detail.value2.items.map(\.position).max() ?? 0) {
                    detail.value2.items.append(item)
                }
            }
        }
        return detail
    }

    /// What an unsent add looks like on screen until the server has it.
    func localItem(for op: PendingOp, after position: Int) -> Item? {
        guard case let .add(name, quantityText, note, categoryId) = op.kind else { return nil }
        return Item(
            id: op.itemId, listId: op.listId, categoryId: categoryId, name: name, quantityText: quantityText,
            note: note, checked: false, addedBy: "", position: position + 1, version: 0, updatedAt: op.at, deleted: false
        )
    }

    private func persist() {
        let disk = Disk(pending: pending, lists: lists, snapshots: snapshots)
        if let data = try? Self.encoder.encode(disk) {
            try? data.write(to: url, options: .atomic)
        }
    }
}
