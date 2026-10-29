import XCTest
@testable import Impressipies

final class FormatTests: XCTestCase {
    func testQuantityUsesFractions() {
        XCTAssertEqual(Format.quantity(1.5), "1 ½")
        XCTAssertEqual(Format.quantity(0.33), "⅓")
        XCTAssertEqual(Format.quantity(2.02), "2")
    }

    func testMinutes() {
        XCTAssertEqual(Format.minutes(45), "45 min")
        XCTAssertEqual(Format.minutes(120), "2 h")
        XCTAssertEqual(Format.minutes(95), "1 h 35 min")
    }

    func testSplitQuantity() {
        XCTAssertEqual(Format.splitQuantity("2 L").qty, "2")
        XCTAssertEqual(Format.splitQuantity("2 L").unit, "L")
        XCTAssertEqual(Format.splitQuantity("a few").qty, "a few")
        XCTAssertEqual(Format.splitQuantity("").qty, "1")
    }

    func testQuickAddParsesLeadingAndTrailingQuantity() {
        XCTAssertEqual(QuickAdd.parse("2 cans chickpeas").name, "chickpeas")
        XCTAssertEqual(QuickAdd.parse("2 cans chickpeas").quantityText, "2 cans")
        XCTAssertEqual(QuickAdd.parse("milk 2L").name, "milk")
        XCTAssertEqual(QuickAdd.parse("milk 2L").quantityText, "2 L")
        XCTAssertEqual(QuickAdd.parse("bread").quantityText, "")
    }
}
