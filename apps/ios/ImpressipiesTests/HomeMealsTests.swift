import XCTest
@testable import Impressipies

final class HomeMealsTests: XCTestCase {
    func testHomeKeepsLunchAndDinnerOnly() {
        let meals = [
            meal("2026-11-13", .breakfast, "Oats"),
            meal("2026-11-13", .lunch, "Soup"),
            meal("2026-11-13", .dinner, "Lemon chicken", recipeId: "r1"),
            meal("2026-11-13", .extra, "Cookies"),
            meal("2026-11-14", .dinner, "Chili"),
        ]

        XCTAssertEqual(HomeMeals.entry(on: "2026-11-13", slot: .lunch, in: meals)?.title, "Soup")
        XCTAssertEqual(HomeMeals.entry(on: "2026-11-13", slot: .dinner, in: meals)?.recipeId, "r1")
        XCTAssertEqual(HomeMeals.entry(on: "2026-11-14", slot: .dinner, in: meals)?.title, "Chili")
        XCTAssertNil(HomeMeals.entry(on: "2026-11-13", slot: .breakfast, in: meals))
        XCTAssertNil(HomeMeals.entry(on: "2026-11-13", slot: .extra, in: meals))
        XCTAssertNil(HomeMeals.entry(on: "2026-11-14", slot: .lunch, in: meals))
    }

    private func meal(_ date: String, _ slot: MealSlot, _ title: String, recipeId: String? = nil) -> MealPlanEntry {
        MealPlanEntry(
            id: "\(date)-\(slot.rawValue)",
            planDate: date,
            slot: slot,
            title: title,
            recipeId: recipeId,
            completed: false,
            notes: "",
            createdAt: Date(timeIntervalSince1970: 0),
            updatedAt: Date(timeIntervalSince1970: 0)
        )
    }
}
