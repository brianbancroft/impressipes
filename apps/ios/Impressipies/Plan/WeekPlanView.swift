import SwiftUI

/// Sunday–Saturday plan. A meal can link a recipe or stay free text.
struct WeekPlanView: View {
    @Environment(Session.self) private var session
    @State private var sunday = FamilyCalendar.startOfWeek(FamilyCalendar.today())
    @State private var meals: [MealPlanEntry] = []
    @State private var recipes: [RecipeSummary] = []
    @State private var selected: String?
    @State private var draft: Draft?
    @State private var error: String?
    @State private var loaded = false

    private var dates: [String] { FamilyCalendar.week(containing: sunday) }
    private var today: String { FamilyCalendar.today() }
    private var focus: String { selected.flatMap { dates.contains($0) ? $0 : nil } ?? (dates.contains(today) ? today : dates[0]) }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("This week").font(.display(.largeTitle)).foregroundStyle(Palette.ink)
                        Text(FamilyCalendar.rangeLabel(from: dates[0], to: dates[6]))
                            .font(.subheadline.weight(.semibold))
                            .foregroundStyle(Palette.inkMuted)
                    }
                    Spacer()
                    Button { shift(-7) } label: { Image(systemName: "chevron.left") }
                        .buttonStyle(RoundIconButtonStyle(fill: Palette.surface2))
                        .accessibilityLabel("Previous week")
                    Button { shift(7) } label: { Image(systemName: "chevron.right") }
                        .buttonStyle(RoundIconButtonStyle(fill: Palette.surface2))
                        .accessibilityLabel("Next week")
                }

                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 8) {
                        ForEach(dates, id: \.self) { date in
                            let on = date == focus
                            Button { selected = date } label: {
                                VStack(spacing: 0) {
                                    Text(FamilyCalendar.shortWeekday(date).prefix(2)).font(.caption.weight(.bold))
                                    Text(FamilyCalendar.dayNumber(date)).font(.system(.title3, design: .rounded, weight: .heavy))
                                }
                                .foregroundStyle(on || date == today ? Palette.stickerInk : Palette.ink)
                                .frame(width: 48, height: 56)
                                .background(Circle().fill(on || date == today ? Palette.butter : Palette.surface2).padding(2))
                                .overlay(Circle().strokeBorder(Palette.outline, lineWidth: on ? 2.5 : 0))
                            }
                            .buttonStyle(.plain)
                            .accessibilityLabel(FamilyCalendar.longWeekday(date))
                            .accessibilityAddTraits(on ? .isSelected : [])
                        }
                    }
                }

                if !loaded && error == nil {
                    ProgressView().frame(maxWidth: .infinity).padding(.top, 24)
                } else if let error, meals.isEmpty {
                    StateMessage(emoji: "🫠", title: "Couldn't load the week", message: error) { Task { await load() } }
                } else {
                    dayBody
                }
            }
            .padding(20)
        }
        .pageBackground()
        .navigationTitle("This week")
        .navigationBarTitleDisplayMode(.inline)
        .task(id: sunday) { await load() }
        .sheet(item: $draft) { draft in
            MealEditorSheet(draft: draft, recipes: recipes) { await load() }
        }
    }

    private var dayBody: some View {
        let mine = meals.filter { $0.planDate == focus }
        let done = mine.filter(\.completed).count
        return VStack(alignment: .leading, spacing: 12) {
            Text("\(FamilyCalendar.longWeekday(focus)) · \(done) of \(mine.count) done")
                .font(.subheadline.weight(.semibold))
                .foregroundStyle(Palette.inkMuted)
            ForEach(MealSlot.ordered, id: \.self) { slot in
                if let meal = mine.first(where: { $0.slot == slot }) {
                    mealCard(meal)
                } else {
                    Button { draft = Draft(date: focus, slot: slot, title: "", recipeId: nil, entryId: nil) } label: {
                        Text(slot.emptyLabel)
                            .font(.body.weight(.semibold))
                            .foregroundStyle(Palette.inkMuted)
                            .frame(maxWidth: .infinity, minHeight: 64, alignment: .leading)
                            .padding(.horizontal, 16)
                            .background(RoundedRectangle(cornerRadius: 16, style: .continuous).strokeBorder(Palette.line, style: StrokeStyle(lineWidth: 2, dash: [6])))
                    }
                    .buttonStyle(.plain)
                }
            }
        }
    }

    private func mealCard(_ meal: MealPlanEntry) -> some View {
        HStack(alignment: .center, spacing: 12) {
            Button { draft = Draft(date: meal.planDate, slot: meal.slot, title: meal.title, recipeId: meal.recipeId, entryId: meal.id) } label: {
                VStack(alignment: .leading, spacing: 4) {
                    Text(meal.slot.label.uppercased())
                        .font(.caption.weight(.bold))
                        .foregroundStyle(Palette.inkMuted)
                    Text(meal.title)
                        .font(.title3.weight(.heavy))
                        .foregroundStyle(meal.completed ? Palette.good : Palette.ink)
                    if let recipe = meal.recipe {
                        Text(recipe.title == meal.title ? "recipe" : recipe.title)
                            .font(.subheadline.weight(.semibold))
                            .foregroundStyle(Palette.stickerInk)
                            .padding(.horizontal, 8)
                            .padding(.vertical, 2)
                            .background(Capsule().fill(Palette.butter))
                    } else {
                        Text("no recipe · free-text")
                            .font(.footnote.weight(.semibold))
                            .foregroundStyle(Palette.inkMuted)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .buttonStyle(.plain)
            Button { Task { await toggle(meal) } } label: {
                Image(systemName: meal.completed ? "checkmark.circle.fill" : "circle")
                    .font(.title2)
                    .foregroundStyle(meal.completed ? Palette.good : Palette.inkMuted)
            }
            .buttonStyle(.plain)
            .accessibilityLabel(meal.completed ? "Mark \(meal.title) not done" : "Mark \(meal.title) done")
        }
        .padding(14)
        .sticker(radius: 18, shadow: meal.completed ? 0 : 3)
    }

    private func toggle(_ meal: MealPlanEntry) async {
        do {
            _ = try await session.api.updateMeal(meal.id, completed: !meal.completed)
            await load()
        } catch {
            self.error = APIError.from(error).message
        }
    }

    private func shift(_ days: Int) {
        sunday = FamilyCalendar.addDays(sunday, days)
        selected = nil
    }

    private func load() async {
        let box = (try? await session.api.recipes(query: nil, tag: nil, mark: nil)) ?? recipes
        do {
            meals = try await session.api.mealPlan(from: dates[0], to: dates[6])
            recipes = box
            error = nil
        } catch {
            self.error = APIError.from(error).message
        }
        loaded = true
    }
}

struct Draft: Identifiable {
    var id: String { "\(date)-\(slot.rawValue)" }
    var date: String
    var slot: MealSlot
    var title: String
    var recipeId: String?
    var entryId: String?
}

struct MealEditorSheet: View {
    @Environment(Session.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State var draft: Draft
    let recipes: [RecipeSummary]
    var onSaved: () async -> Void

    @State private var query = ""
    @State private var busy = false
    @State private var error: String?

    private var matches: [RecipeSummary] {
        let q = query.trimmingCharacters(in: .whitespaces).lowercased()
        let pool = q.isEmpty ? recipes : recipes.filter { $0.title.lowercased().contains(q) }
        return Array(pool.prefix(8))
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    Text("Link a recipe, or leave it blank for a meal you make from memory, like tacos.")
                        .font(.subheadline)
                        .foregroundStyle(Palette.inkMuted)
                    Text("Title").font(.subheadline.weight(.bold))
                    TextField("Tacos", text: $draft.title)
                        .padding(14)
                        .sticker(radius: 16, shadow: 3)
                    Text("Link a recipe").font(.subheadline.weight(.bold))
                    TextField("Search recipes", text: $query)
                        .padding(14)
                        .sticker(radius: 16, shadow: 0)
                    Button { pick(nil) } label: {
                        Text("No recipe · free text")
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding(12)
                            .background(RoundedRectangle(cornerRadius: 12).fill(draft.recipeId == nil ? Palette.butter : Palette.surface2))
                    }
                    .buttonStyle(.plain)
                    .foregroundStyle(draft.recipeId == nil ? Palette.stickerInk : Palette.ink)
                    ForEach(matches, id: \.id) { recipe in
                        Button { pick(recipe) } label: {
                            Text(recipe.title)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .padding(12)
                                .background(RoundedRectangle(cornerRadius: 12).fill(draft.recipeId == recipe.id ? Palette.butter : Palette.surface))
                        }
                        .buttonStyle(.plain)
                        .foregroundStyle(draft.recipeId == recipe.id ? Palette.stickerInk : Palette.ink)
                    }
                    if let error { Text(error).foregroundStyle(Palette.danger) }
                    Button(busy ? "Saving…" : "Save") { Task { await save() } }
                        .buttonStyle(PopButtonStyle())
                        .disabled(busy || draft.title.trimmingCharacters(in: .whitespaces).isEmpty)
                    if draft.entryId != nil {
                        Button("Remove meal", role: .destructive) { Task { await remove() } }
                            .frame(maxWidth: .infinity)
                    }
                }
                .padding(20)
            }
            .pageBackground()
            .navigationTitle(draft.entryId == nil ? "Add \(draft.slot.label.lowercased())" : "Edit \(draft.slot.label.lowercased())")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar { ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } } }
        }
        .presentationDetents([.large])
    }

    private func pick(_ recipe: RecipeSummary?) {
        let previous = recipes.first { $0.id == draft.recipeId }?.title
        draft.recipeId = recipe?.id
        let trimmed = draft.title.trimmingCharacters(in: .whitespaces)
        if trimmed.isEmpty || trimmed == previous {
            draft.title = recipe?.title ?? draft.title
        }
    }

    private func save() async {
        busy = true
        defer { busy = false }
        do {
            _ = try await session.api.upsertMeal(
                planDate: draft.date, slot: draft.slot,
                title: draft.title.trimmingCharacters(in: .whitespaces),
                recipeId: draft.recipeId
            )
            await onSaved()
            dismiss()
        } catch {
            self.error = APIError.from(error).message
        }
    }

    private func remove() async {
        guard let id = draft.entryId else { return }
        busy = true
        defer { busy = false }
        do {
            try await session.api.deleteMeal(id)
            await onSaved()
            dismiss()
        } catch {
            self.error = APIError.from(error).message
        }
    }
}
