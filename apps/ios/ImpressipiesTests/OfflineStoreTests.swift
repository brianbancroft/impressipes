import XCTest
@testable import Impressipies

@MainActor
final class OfflineStoreTests: XCTestCase {
    private func store() -> OfflineStore {
        OfflineStore(url: FileManager.default.temporaryDirectory.appending(path: "offline-\(UUID().uuidString).json"))
    }

    func testLatestCheckWins() {
        let s = store()
        s.enqueue(PendingOp(listId: "l", itemId: "a", kind: .check(true), at: .now))
        s.enqueue(PendingOp(listId: "l", itemId: "a", kind: .check(false), at: .now))
        XCTAssertEqual(s.pending.count, 1)
        XCTAssertEqual(s.pending.first?.kind, .check(false))
    }

    func testAddAndCheckBothStayQueued() {
        let s = store()
        s.enqueue(PendingOp(listId: "l", itemId: "a", kind: .add(name: "Milk", quantityText: "", note: "", categoryId: nil), at: .now))
        s.enqueue(PendingOp(listId: "l", itemId: "a", kind: .check(true), at: .now))
        XCTAssertEqual(s.pending.count, 2)
        XCTAssertEqual(s.pendingIds(in: "l"), ["a"])
    }

    func testQueueSurvivesRestart() {
        let url = FileManager.default.temporaryDirectory.appending(path: "offline-\(UUID().uuidString).json")
        OfflineStore(url: url).enqueue(PendingOp(listId: "l", itemId: "a", kind: .check(true), at: .now))
        XCTAssertEqual(OfflineStore(url: url).pending.count, 1)
    }

    func testLocalItemForUnsentAdd() {
        let s = store()
        let op = PendingOp(listId: "l", itemId: "a", kind: .add(name: "Milk", quantityText: "2 L", note: "", categoryId: nil), at: .now)
        let item = s.localItem(for: op, after: 4)
        XCTAssertEqual(item?.name, "Milk")
        XCTAssertEqual(item?.position, 5)
        XCTAssertNil(s.localItem(for: PendingOp(listId: "l", itemId: "a", kind: .check(true), at: .now), after: 0))
    }
}
