import SwiftUI

/// Lunch and dinner for the next seven days, with a bill reminder board underneath.
struct HomeView: View {
    @Environment(Session.self) private var session
    @State private var meals: [MealPlanEntry] = []
    @State private var error: String?
    @State private var loaded = false

    private var today: String { FamilyCalendar.today() }
    private var dates: [String] { FamilyCalendar.upcoming(from: today) }

    var body: some View {
        NavigationStack {
            HomeScreen(dates: dates, today: today, meals: meals, loaded: loaded, error: error) {
                await load()
            }
            .navigationTitle("Home")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar(.hidden, for: .navigationBar)
            .navigationDestination(for: String.self) { id in
                RecipeDetailView(recipeId: id)
            }
            .task { await load() }
        }
    }

    private func load() async {
        #if DEBUG
        // Simulator-only: `xcrun simctl launch … -previewHome` shows Home without a session.
        if ProcessInfo.processInfo.arguments.contains("-previewHome") {
            meals = HomePreviewMeal.sample
            error = nil
            loaded = true
            return
        }
        #endif
        let end = dates.last ?? today
        do {
            meals = try await session.api.mealPlan(from: today, to: end)
            error = nil
        } catch {
            self.error = APIError.from(error).message
        }
        loaded = true
    }
}

/// Lunch and dinner only. Breakfast and the extra slot stay on the week screen.
enum HomeMeals {
    static func entry(on date: String, slot: MealSlot, in meals: [MealPlanEntry]) -> MealPlanEntry? {
        guard slot == .lunch || slot == .dinner else { return nil }
        return meals.first { $0.planDate == date && $0.slot == slot }
    }
}

private struct HomeScreen: View {
    var dates: [String]
    var today: String
    var meals: [MealPlanEntry]
    var loaded: Bool
    var error: String?
    var reload: () async -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            header
            week
            BillReminderBoard()
        }
        .padding(.horizontal, 20)
        .padding(.top, 8)
        // Clears the floating tab bar, including the sticker shadow.
        .safeAreaPadding(.bottom, 12)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
        .pageBackground()
    }

    private var header: some View {
        HStack(alignment: .firstTextBaseline, spacing: 12) {
            VStack(alignment: .leading, spacing: 2) {
                Text("Home").font(.display(.largeTitle)).foregroundStyle(Palette.ink)
                Text("Lunch and dinner · next 7 days")
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(Palette.inkMuted)
            }
            Spacer(minLength: 8)
            NavigationLink {
                WeekPlanView()
            } label: {
                Text("Open week")
                    .font(.subheadline.weight(.heavy))
                    .foregroundStyle(Palette.ink)
                    .padding(.horizontal, 14)
                    .padding(.vertical, 8)
                    .background(Capsule().fill(Palette.surface2))
                    .overlay(Capsule().strokeBorder(Palette.outline, lineWidth: 2))
            }
            .accessibilityHint("Opens the Sunday to Saturday plan")
        }
    }

    @ViewBuilder
    private var week: some View {
        if !loaded && error == nil {
            ProgressView()
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else if let error, meals.isEmpty {
            StateMessage(emoji: "🫠", title: "Couldn't load home", message: error) {
                Task { await reload() }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else {
            weekPanel
        }
    }

    private var weekPanel: some View {
        ScrollView {
            VStack(spacing: 0) {
                columnHeader
                Rectangle().fill(Palette.line).frame(height: 1)
                ForEach(Array(dates.enumerated()), id: \.element) { index, date in
                    DayLine(
                        date: date,
                        isToday: date == today,
                        lunch: HomeMeals.entry(on: date, slot: .lunch, in: meals),
                        dinner: HomeMeals.entry(on: date, slot: .dinner, in: meals)
                    )
                    if index < dates.count - 1 {
                        Rectangle()
                            .fill(Palette.line)
                            .frame(height: 1)
                            .padding(.leading, 76)
                    }
                }
            }
            .sticker(radius: 18, shadow: 3)
        }
        .scrollClipDisabled()
        .refreshable { await reload() }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
    }

    private var columnHeader: some View {
        HStack(spacing: 8) {
            Color.clear.frame(width: 56, height: 1).accessibilityHidden(true)
            Text("Lunch")
                .frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityAddTraits(.isHeader)
            Text("Dinner")
                .frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityAddTraits(.isHeader)
        }
        .font(.caption.weight(.bold))
        .foregroundStyle(Palette.inkMuted)
        .textCase(.uppercase)
        .padding(.horizontal, 12)
        .padding(.top, 12)
        .padding(.bottom, 6)
        .accessibilityElement(children: .contain)
    }
}

private struct DayLine: View {
    let date: String
    let isToday: Bool
    let lunch: MealPlanEntry?
    let dinner: MealPlanEntry?

    var body: some View {
        HStack(spacing: 8) {
            Text(FamilyCalendar.shortWeekday(date))
                .font(.system(.subheadline, design: .rounded, weight: .heavy))
                .foregroundStyle(isToday ? Palette.stickerInk : Palette.ink)
                .padding(.horizontal, 8)
                .padding(.vertical, 4)
                .background(Capsule().fill(isToday ? Palette.butter : Color.clear))
                .frame(width: 56, alignment: .center)
                .accessibilityLabel(isToday ? "\(FamilyCalendar.longWeekday(date)), today" : FamilyCalendar.longWeekday(date))

            mealValue(lunch, slot: .lunch)
            mealValue(dinner, slot: .dinner)
        }
        .padding(.horizontal, 12)
        .frame(minHeight: 44)
    }

    @ViewBuilder
    private func mealValue(_ meal: MealPlanEntry?, slot: MealSlot) -> some View {
        let spoken = "\(slot.label), \(meal?.title ?? "nothing planned")"
        Group {
            if let meal {
                let title = Text(meal.title)
                    .font(.subheadline.weight(.heavy))
                    .foregroundStyle(meal.completed ? Palette.good : Palette.ink)
                    .lineLimit(1)
                    .frame(maxWidth: .infinity, alignment: .leading)
                if let id = meal.recipeId {
                    NavigationLink(value: id) { title }
                        .buttonStyle(.plain)
                        .accessibilityLabel(spoken)
                        .accessibilityHint("Opens recipe")
                } else {
                    title.accessibilityLabel(spoken)
                }
            } else {
                Text("—")
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(Palette.inkMuted)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityLabel(spoken)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

#if DEBUG
#Preview("Home") {
    let today = FamilyCalendar.today()
    NavigationStack {
        HomeScreen(
            dates: FamilyCalendar.upcoming(from: today),
            today: today,
            meals: HomePreviewMeal.sample,
            loaded: true,
            error: nil,
            reload: {}
        )
    }
    .preferredColorScheme(.dark)
}

#Preview("Home, empty") {
    let today = FamilyCalendar.today()
    NavigationStack {
        HomeScreen(
            dates: FamilyCalendar.upcoming(from: today),
            today: today,
            meals: [],
            loaded: true,
            error: nil,
            reload: {}
        )
    }
    .preferredColorScheme(.dark)
}

private enum HomePreviewMeal {
    static var sample: [MealPlanEntry] {
        let today = FamilyCalendar.today()
        return [
            make(date: today, slot: .breakfast, title: "Oats"),
            make(date: today, slot: .dinner, title: "Lemon chicken", recipeId: "preview-chicken"),
            make(date: FamilyCalendar.addDays(today, 1), slot: .lunch, title: "Leftover soup"),
            make(date: FamilyCalendar.addDays(today, 2), slot: .dinner, title: "Chili", recipeId: "preview-chili", completed: true),
            make(date: FamilyCalendar.addDays(today, 2), slot: .extra, title: "Cookies"),
        ]
    }

    static func make(date: String, slot: MealSlot, title: String, recipeId: String? = nil, completed: Bool = false) -> MealPlanEntry {
        MealPlanEntry(
            id: "\(date)-\(slot.rawValue)",
            planDate: date,
            slot: slot,
            title: title,
            recipeId: recipeId,
            completed: completed,
            notes: "",
            createdAt: Date(timeIntervalSince1970: 0),
            updatedAt: Date(timeIntervalSince1970: 0)
        )
    }
}
#endif
