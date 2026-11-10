import SwiftUI

/// Next seven days, starting today, plus ideas drawn from the family's own recipes and past plans.
struct HomeView: View {
    @Environment(Session.self) private var session
    @State private var meals: [MealPlanEntry] = []
    @State private var history: [MealPlanEntry] = []
    @State private var recipes: [RecipeSummary] = []
    @State private var error: String?
    @State private var loaded = false

    private var today: String { FamilyCalendar.today() }
    private var dates: [String] { FamilyCalendar.upcoming(from: today) }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    HStack(alignment: .firstTextBaseline) {
                        VStack(alignment: .leading, spacing: 2) {
                            Text("Home").font(.display(.largeTitle)).foregroundStyle(Palette.ink)
                            Text("Next 7 days · from your meal plan")
                                .font(.subheadline.weight(.semibold))
                                .foregroundStyle(Palette.inkMuted)
                        }
                        Spacer()
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
                    }

                    if !loaded && error == nil {
                        ProgressView().frame(maxWidth: .infinity).padding(.top, 40)
                    } else if let error, meals.isEmpty && recipes.isEmpty {
                        StateMessage(emoji: "🫠", title: "Couldn't load home", message: error) { Task { await load() } }
            } else {
                ForEach(dates, id: \.self) { date in
                    dayCard(date, ideas: planIdeas[date] ?? [])
                }
            }
                }
                .padding(20)
            }
            .pageBackground()
            .navigationBarTitleDisplayMode(.inline)
            .navigationDestination(for: String.self) { id in
                RecipeDetailView(recipeId: id)
            }
            .refreshable { await load() }
            .task { await load() }
        }
    }

    private var planIdeas: [String: [MealIdea]] {
        var taken = Set(meals.map { $0.recipeId ?? "text:\($0.title.lowercased())" })
        var map: [String: [MealIdea]] = [:]
        for date in dates {
            map[date] = MealIdeas.forDay(date, in: dates, meals: meals, recipes: recipes, history: history, taken: &taken)
        }
        return map
    }

    private func dayCard(_ date: String, ideas: [MealIdea]) -> some View {
        let mine = meals.filter { $0.planDate == date }.sorted { slotRank($0.slot) < slotRank($1.slot) }
        let isToday = date == today
        return VStack(alignment: .leading, spacing: 8) {
            Text(isToday ? "Today · \(FamilyCalendar.shortWeekday(date))" : "\(FamilyCalendar.shortWeekday(date)) \(FamilyCalendar.dayNumber(date))")
                .font(.subheadline.weight(.heavy))
                .foregroundStyle(isToday ? Palette.stickerInk : Palette.inkMuted)
            if mine.isEmpty {
                Text("— nothing planned —")
                    .font(.body.weight(.semibold))
                    .foregroundStyle(Palette.ink)
            } else {
                ForEach(mine, id: \.id) { meal in
                    mealLine(meal)
                }
            }
            if !ideas.isEmpty {
                suggestionLine(ideas, empty: mine.isEmpty)
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .sticker(radius: 18, shadow: 3)
    }

    @ViewBuilder
    private func mealLine(_ meal: MealPlanEntry) -> some View {
        let text = "\(meal.slot.label): \(meal.title)\(meal.recipeId == nil ? " (free-text)" : "")"
        if let id = meal.recipeId {
            NavigationLink(value: id) {
                Text(text).font(.body.weight(.semibold)).foregroundStyle(Palette.ink).frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.plain)
        } else {
            Text(text).font(.body.weight(.semibold)).foregroundStyle(Palette.ink)
        }
    }

    private func suggestionLine(_ ideas: [MealIdea], empty: Bool) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            if empty {
                tryRow(ideas.filter { $0.slot == .dinner })
                if let breakfast = ideas.first(where: { $0.slot == .breakfast }) {
                    Button { Task { await accept(breakfast) } } label: {
                        Text("Breakfast idea: \(breakfast.title)")
                            .font(.subheadline.weight(.semibold))
                            .foregroundStyle(Palette.good)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                    .buttonStyle(.plain)
                }
            } else {
                ForEach(ideas) { idea in
                    Button { Task { await accept(idea) } } label: {
                        Text(idea.slot == .breakfast ? "Breakfast idea: \(idea.title)" : "Suggestion: \(idea.title)")
                            .font(.subheadline.weight(.semibold))
                            .foregroundStyle(Palette.good)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                    .buttonStyle(.plain)
                }
            }
        }
    }

    @ViewBuilder
    private func tryRow(_ ideas: [MealIdea]) -> some View {
        if !ideas.isEmpty {
            FlowLayout(spacing: 6) {
                Text("Try:").font(.subheadline.weight(.semibold)).foregroundStyle(Palette.good)
                ForEach(Array(ideas.enumerated()), id: \.element.id) { index, idea in
                    if index > 0 { Text("· or").font(.subheadline.weight(.semibold)).foregroundStyle(Palette.good) }
                    Button(idea.title) { Task { await accept(idea) } }
                        .font(.subheadline.weight(.heavy))
                        .foregroundStyle(Palette.good)
                        .buttonStyle(.plain)
                }
            }
        }
    }

    private func accept(_ idea: MealIdea) async {
        guard let date = planIdeas.first(where: { $0.value.contains(idea) })?.key else { return }
        do {
            _ = try await session.api.upsertMeal(planDate: date, slot: idea.slot, title: idea.title, recipeId: idea.recipeId)
            await load()
        } catch {
            self.error = APIError.from(error).message
        }
    }

    private func slotRank(_ slot: MealSlot) -> Int {
        MealSlot.ordered.firstIndex(of: slot) ?? 9
    }

    private func load() async {
        let start = FamilyCalendar.addDays(today, -56)
        let end = dates.last ?? today
        async let plan = session.api.mealPlan(from: start, to: end)
        async let box = session.api.recipes(query: nil, tag: nil, mark: nil)
        do {
            let (entries, found) = try await (plan, box)
            history = entries.filter { $0.planDate < today }
            meals = entries.filter { $0.planDate >= today }
            recipes = found
            error = nil
        } catch {
            self.error = APIError.from(error).message
        }
        loaded = true
    }
}
