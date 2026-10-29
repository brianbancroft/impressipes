import SwiftUI

/// One recipe: hero, meta stickers, servings scaler, ingredients / steps, and a
/// sticky Add to list bar. Favourite and Want to make live in the toolbar.
struct RecipeDetailView: View {
    let recipeId: String
    var openLists: () -> Void = {}
    var onMarksChanged: ([RecipeMark]) -> Void = { _ in }

    @Environment(Session.self) private var session
    @State private var recipe: Recipe?
    @State private var error: String?
    @State private var servings: Double = 0
    @State private var tab = Tab.ingredients
    @State private var ticked: Set<Int> = []
    @State private var addingToList = false
    @State private var toast: Toast?

    enum Tab: String, CaseIterable { case ingredients = "Ingredients", steps = "Steps" }

    var body: some View {
        Group {
            if let recipe {
                content(recipe)
            } else if let error {
                StateMessage(emoji: "🫠", title: "Couldn't load this recipe", message: error) { Task { await load() } }
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .pageBackground()
        .navigationBarTitleDisplayMode(.inline)
        .task { if recipe == nil { await load() } }
        .toast($toast)
    }

    private var factor: Double {
        guard let base = recipe?.summary.servings, base > 0, servings > 0 else { return 1 }
        return servings / base
    }

    private func content(_ r: Recipe) -> some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                RemoteImage(path: r.heroPath) { RecipeArt(title: r.title) }
                    .frame(height: 230)
                    .frame(maxWidth: .infinity)
                    .clipShape(RoundedRectangle(cornerRadius: 24, style: .continuous))
                    .sticker(radius: 24, shadow: 5)

                VStack(alignment: .leading, spacing: 8) {
                    Text(r.title)
                        .font(.display(.largeTitle))
                        .foregroundStyle(Palette.ink)
                    if !r.summary.summary.isEmpty {
                        Text(r.summary.summary).foregroundStyle(Palette.inkMuted)
                    }
                    if let source = sourceHost(r) {
                        Text("From \(source)").font(.footnote.weight(.semibold)).foregroundStyle(Palette.inkMuted)
                    }
                }

                FlowLayout(spacing: 8) {
                    if let p = r.summary.prepMinutes, p > 0 { Chip(label: "Prep \(Format.minutes(p))", systemImage: "hourglass", fill: Palette.butter) }
                    if let c = r.summary.cookMinutes, c > 0 { Chip(label: "Cook \(Format.minutes(c))", systemImage: "flame", fill: Palette.tangerine) }
                    ForEach(r.summary.tags, id: \.self) { t in Chip(label: t, systemImage: "tag", fill: Palette.sky) }
                }

                if (r.summary.servings ?? 0) > 0 {
                    StepperPill(
                        label: "Servings",
                        value: Format.quantity(servings),
                        decrement: { withAnimation { servings = max(1, servings - 1) } },
                        increment: { withAnimation { servings += 1 } }
                    )
                }

                Picker("Show", selection: $tab) {
                    ForEach(Tab.allCases, id: \.self) { Text($0.rawValue).tag($0) }
                }
                .pickerStyle(.segmented)

                switch tab {
                case .ingredients: ingredients(r)
                case .steps: steps(r)
                }

                if !r.notes.isEmpty {
                    VStack(alignment: .leading, spacing: 6) {
                        Scribble(text: "notes")
                        Text(r.notes).foregroundStyle(Palette.stickerInk)
                    }
                    .padding(16)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .sticker(Palette.butter.tint(0.6), radius: 18, shadow: 3)
                    .rotationEffect(.degrees(-0.8))
                }
            }
            .padding(20)
            .padding(.bottom, 80)
        }
        .safeAreaInset(edge: .bottom) {
            Button { addingToList = true } label: {
                Label("Add to list", systemImage: "cart.badge.plus")
            }
            .buttonStyle(PopButtonStyle())
            .disabled(r.ingredients.isEmpty)
            .padding(.horizontal, 20)
            .padding(.vertical, 10)
            .background(Palette.bg.opacity(0.95))
        }
        .toolbar {
            ToolbarItemGroup(placement: .primaryAction) {
                markButton(r, .want_to_make, on: "bookmark.fill", off: "bookmark", label: "Want to make")
                markButton(r, .favourite, on: "heart.fill", off: "heart", label: "Favourite")
            }
        }
        .sheet(isPresented: $addingToList) {
            AddToListSheet(recipe: r, servings: servings) { message in
                toast = Toast(message: message, actionLabel: "View list", action: openLists)
            }
        }
    }

    private func ingredients(_ r: Recipe) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            if r.ingredients.isEmpty {
                Text("No ingredients listed.").foregroundStyle(Palette.inkMuted)
            }
            ingredientRows(r)
        }
    }

    /// Tickable ingredient rows. The Ingredients tab and the gather step share `ticked`.
    @ViewBuilder
    private func ingredientRows(_ r: Recipe) -> some View {
        ForEach(Array(r.ingredients.enumerated()), id: \.offset) { index, ing in
            let section = ing.section ?? ""
            if !section.isEmpty, index == 0 || r.ingredients[index - 1].section != ing.section {
                Text(section).font(.headline.weight(.heavy)).foregroundStyle(Palette.ink).padding(.top, 6)
            }
            let parts = Format.ingredient(ing, factor: factor)
            let done = ticked.contains(index)
            Button {
                if done { ticked.remove(index) } else { ticked.insert(index) }
            } label: {
                HStack(spacing: 12) {
                    TickBox(checked: done)
                    if let qty = parts.qty {
                        QuantityBadge(qty: qty, unit: parts.unit, muted: done)
                    }
                    Text(parts.text)
                        .font(.body.weight(.medium))
                        .textCase(.uppercase)
                        .strikethrough(done)
                        .foregroundStyle(done ? Palette.inkMuted : Palette.ink)
                        .multilineTextAlignment(.leading)
                    Spacer(minLength: 0)
                }
                .padding(8)
                .frame(minHeight: 56)
                .sticker(radius: 16, shadow: done ? 0 : 3)
            }
            .buttonStyle(.plain)
            .accessibilityAddTraits(done ? .isSelected : [])
        }
    }

    /// Step 1 is always gathering the ingredients, as a checklist; the recipe's own steps follow.
    private func steps(_ r: Recipe) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 12) {
                stepNumber(0)
                VStack(alignment: .leading, spacing: 10) {
                    HStack(alignment: .firstTextBaseline) {
                        Text("Gather your ingredients").font(.headline).foregroundStyle(Palette.ink)
                        Spacer()
                        if !r.ingredients.isEmpty {
                            Text("\(ticked.count) of \(r.ingredients.count) ready")
                                .font(.footnote.weight(.bold))
                                .foregroundStyle(Palette.inkMuted)
                        }
                    }
                    if r.ingredients.isEmpty {
                        Text("No ingredients listed. On to the steps!").foregroundStyle(Palette.inkMuted)
                    }
                    ingredientRows(r)
                }
                .padding(.top, 8)
            }
            if r.steps.isEmpty {
                Text("No steps written down.").foregroundStyle(Palette.inkMuted)
            }
            ForEach(Array(r.steps.enumerated()), id: \.offset) { offset, step in
                let index = offset + 1
                HStack(alignment: .top, spacing: 12) {
                    stepNumber(index)
                    VStack(alignment: .leading, spacing: 6) {
                        Text(step.text).foregroundStyle(Palette.ink)
                        if let t = step.timerSeconds {
                            Chip(label: Format.minutes(max(1, t / 60)), systemImage: "timer", fill: Palette.mint)
                        }
                    }
                    .padding(.top, 8)
                }
            }
        }
    }

    private func stepNumber(_ index: Int) -> some View {
        Text("\(index + 1)")
            .font(.system(.headline, design: .rounded, weight: .black))
            .foregroundStyle(Palette.stickerInk)
            .frame(width: 38, height: 38)
            .background(Circle().fill(Palette.stickers[index % Palette.stickers.count]))
            .overlay(Circle().strokeBorder(Palette.outline, lineWidth: 2))
    }

    private func markButton(_ r: Recipe, _ mark: RecipeMark, on: String, off: String, label: String) -> some View {
        let active = r.summary.has(mark)
        return Button {
            UIImpactFeedbackGenerator(style: .medium).impactOccurred()
            setMark(mark, !active)
        } label: {
            Image(systemName: active ? on : off)
        }
        .accessibilityLabel(label)
        .accessibilityAddTraits(active ? .isSelected : [])
    }

    private func sourceHost(_ r: Recipe) -> String? {
        guard let s = r.sourceUrl, let host = URL(string: s)?.host() else { return nil }
        return host.hasPrefix("www.") ? String(host.dropFirst(4)) : host
    }

    private func load() async {
        do {
            let r = try await session.api.recipe(recipeId)
            recipe = r
            servings = r.summary.servings ?? 0
            error = nil
        } catch {
            self.error = APIError.from(error).message
        }
    }

    private func setMark(_ mark: RecipeMark, _ on: Bool) {
        guard var r = recipe else { return }
        let before = r.value1.marks
        r.value1.marks = on ? before + [mark] : before.filter { $0 != mark }
        recipe = r
        onMarksChanged(r.value1.marks)
        Task {
            do {
                try await session.api.setMark(mark, on: recipeId, on)
                toast = Toast(message: on ? (mark == .favourite ? "Added to faves" : "Saved for later") : (mark == .favourite ? "Removed from faves" : "Removed from want to make"))
            } catch {
                recipe?.value1.marks = before
                onMarksChanged(before)
                toast = Toast(message: APIError.from(error).message)
            }
        }
    }
}
