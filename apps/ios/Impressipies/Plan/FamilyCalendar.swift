import Foundation

/// The household plans in America/Vancouver. Weeks run Sunday through Saturday.
enum FamilyCalendar {
    static let timeZone = TimeZone(identifier: "America/Vancouver")!

    static var calendar: Calendar {
        var c = Calendar(identifier: .gregorian)
        c.timeZone = timeZone
        c.firstWeekday = 1
        return c
    }

    static let monthDay: DateFormatter = {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US")
        f.timeZone = timeZone
        f.setLocalizedDateFormatFromTemplate("MMMd")
        return f
    }()

    private static let weekday: DateFormatter = {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US")
        f.timeZone = timeZone
        f.setLocalizedDateFormatFromTemplate("EEE")
        return f
    }()

    private static let longWeekday: DateFormatter = {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US")
        f.timeZone = timeZone
        f.setLocalizedDateFormatFromTemplate("EEEE")
        return f
    }()

    static func iso(_ date: Date) -> String {
        let p = calendar.dateComponents([.year, .month, .day], from: date)
        return String(format: "%04d-%02d-%02d", p.year ?? 0, p.month ?? 1, p.day ?? 1)
    }

    static func date(_ iso: String) -> Date {
        let p = iso.split(separator: "-").compactMap { Int($0) }
        guard p.count == 3 else { return Date() }
        return calendar.date(from: DateComponents(year: p[0], month: p[1], day: p[2])) ?? Date()
    }

    static func today() -> String { iso(Date()) }

    static func addDays(_ iso: String, _ n: Int) -> String {
        let next = calendar.date(byAdding: .day, value: n, to: date(iso)) ?? Date()
        return self.iso(next)
    }

    /// Sunday of the week containing `iso`.
    static func startOfWeek(_ iso: String) -> String {
        let weekday = calendar.component(.weekday, from: date(iso))
        return addDays(iso, -(weekday - 1))
    }

    static func week(containing iso: String) -> [String] {
        let start = startOfWeek(iso)
        return (0..<7).map { addDays(start, $0) }
    }

    static func upcoming(from iso: String, days: Int = 7) -> [String] {
        (0..<days).map { addDays(iso, $0) }
    }

    static func shortWeekday(_ iso: String) -> String { weekday.string(from: date(iso)) }
    static func longWeekday(_ iso: String) -> String { longWeekday.string(from: date(iso)) }
    static func dayNumber(_ iso: String) -> String { String(calendar.component(.day, from: date(iso))) }

    static func rangeLabel(from: String, to: String) -> String {
        let a = date(from)
        let b = date(to)
        if calendar.isDate(a, equalTo: b, toGranularity: .month) {
            return "\(monthDay.string(from: a))–\(calendar.component(.day, from: b))"
        }
        return "\(monthDay.string(from: a)) – \(monthDay.string(from: b))"
    }
}

extension MealSlot {
    static let ordered: [MealSlot] = [.breakfast, .lunch, .dinner, .extra]

    var label: String {
        switch self {
        case .breakfast: "Breakfast"
        case .lunch: "Lunch"
        case .dinner: "Dinner"
        case .extra: "Extra"
        }
    }

    var emptyLabel: String {
        self == .extra ? "+ Add a snack or side" : "+ Add \(label.lowercased())"
    }
}

/// A tappable idea derived on device from recipes the family already has and meals they have planned.
struct MealIdea: Identifiable, Hashable {
    var id: String { recipeId ?? "text:\(title.lowercased())" }
    var title: String
    var recipeId: String?
    var slot: MealSlot
}

enum MealIdeas {
    /// Ideas for one day. Dinner suggestions fill an empty dinner, or sit under today's planned dinner.
    /// A breakfast idea appears on the last day of the window when breakfast is still open.
    static func forDay(
        _ date: String,
        in dates: [String],
        meals: [MealPlanEntry],
        recipes: [RecipeSummary],
        history: [MealPlanEntry],
        taken: inout Set<String>
    ) -> [MealIdea] {
        let mine = meals.filter { $0.planDate == date }
        let hasDinner = mine.contains { $0.slot == .dinner }
        let hasBreakfast = mine.contains { $0.slot == .breakfast }
        var out: [MealIdea] = []
        if hasDinner {
            if date == dates.first, let idea = next(.dinner, recipes: recipes, history: history, taken: &taken) {
                out.append(idea)
            }
        } else {
            for _ in 0..<2 {
                guard let idea = next(.dinner, recipes: recipes, history: history, taken: &taken) else { break }
                out.append(idea)
            }
        }
        if !hasBreakfast, date == dates.last, let idea = next(.breakfast, recipes: recipes, history: history, taken: &taken) {
            out.append(idea)
        }
        return out
    }

    private static func next(_ slot: MealSlot, recipes: [RecipeSummary], history: [MealPlanEntry], taken: inout Set<String>) -> MealIdea? {
        for idea in ranked(slot: slot, recipes: recipes, history: history) where taken.insert(idea.id).inserted {
            return idea
        }
        return nil
    }

    private static func ranked(slot: MealSlot, recipes: [RecipeSummary], history: [MealPlanEntry]) -> [MealIdea] {
        var scored: [(MealIdea, Int)] = []
        for r in recipes {
            let breakfast = r.tags.contains { $0.localizedCaseInsensitiveContains("breakfast") }
            if slot == .breakfast, !breakfast, !history.contains(where: { $0.slot == .breakfast && $0.recipeId == r.id }) { continue }
            if slot == .dinner, breakfast { continue }
            var score = 0
            if r.marks.contains(.want_to_make) { score += 4 }
            if r.marks.contains(.favourite) { score += 3 }
            if let cooked = r.lastCookedAt, cooked.timeIntervalSinceNow > -60 * 60 * 24 * 90 { score += 2 }
            if score == 0 { score = 1 }
            scored.append((MealIdea(title: r.title, recipeId: r.id, slot: slot), score))
        }
        var counts: [String: Int] = [:]
        for e in history where e.slot == slot && e.recipeId == nil {
            counts[e.title, default: 0] += 1
        }
        for (title, n) in counts {
            scored.append((MealIdea(title: title, recipeId: nil, slot: slot), n))
        }
        return scored.sorted { $0.1 == $1.1 ? $0.0.title < $1.0.title : $0.1 > $1.1 }.map(\.0)
    }
}
