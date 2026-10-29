import Foundation

/// A check-off the server hasn't acknowledged yet.
struct PendingCheck: Codable, Equatable {
    let listId: String
    let itemId: String
    let checked: Bool
    let at: Date
}

/// A list as last seen on the server.
struct ListSnapshot: Codable {
    let detail: ListDetail
    let syncedAt: Date
}

/// What the watch keeps on disk so it works when the phone and signal are out
/// of reach (IMP-58): the last list contents, and check-offs waiting to be sent.
/// Writes are always accepted here first.
@MainActor
final class OfflineStore {
    static let shared = OfflineStore()

    private(set) var pending: [PendingCheck]
    private(set) var lists: [ShoppingList]
    private var snapshots: [String: ListSnapshot]

    private struct Disk: Codable {
        var pending: [PendingCheck] = []
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

    /// Latest tap wins: crossing an item off and putting it back before it
    /// syncs leaves one entry, not two.
    func enqueue(_ check: PendingCheck) {
        pending.removeAll { $0.itemId == check.itemId }
        pending.append(check)
        persist()
    }

    func acknowledge(_ check: PendingCheck) {
        // A newer tap on the same item stays queued.
        pending.removeAll { $0 == check }
        persist()
    }

    /// Sends queued check-offs, oldest first, and stops at the first one that
    /// can't get through. Returns the server's version of each item it sent.
    func flush(using api: API) async -> [Item] {
        var sent: [Item] = []
        for check in pending {
            do {
                let item = try await api.updateItem(check.itemId, .init(checked: check.checked))
                acknowledge(check)
                // A newer tap on the same item has replaced this one in the queue.
                if !pendingIds(in: check.listId).contains(item.id) { sent.append(item) }
            } catch {
                if APIError.isUnreachable(error) { break }
                // The server turned it down (item gone, say): drop it and let the next refresh correct the row.
                acknowledge(check)
            }
        }
        return sent
    }

    func pendingIds(in listId: String) -> Set<String> {
        Set(pending.filter { $0.listId == listId }.map(\.itemId))
    }

    /// Puts unacknowledged check-offs back on top of what the server sent, so a
    /// refresh doesn't undo a tap that hasn't been delivered yet.
    func overlay(_ detail: ListDetail, listId: String) -> ListDetail {
        var detail = detail
        for p in pending where p.listId == listId {
            if let i = detail.value2.items.firstIndex(where: { $0.id == p.itemId }) {
                detail.value2.items[i].checked = p.checked
                detail.value2.items[i].updatedAt = p.at
            }
        }
        return detail
    }

    private func persist() {
        let disk = Disk(pending: pending, lists: lists, snapshots: snapshots)
        if let data = try? Self.encoder.encode(disk) {
            try? data.write(to: url, options: .atomic)
        }
    }
}
