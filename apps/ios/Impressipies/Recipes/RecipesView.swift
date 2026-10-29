import SwiftUI

/// The recipe box: search, sticker filter chips, two-column cards. Read only on
/// iOS; recipes are written on the web.
struct RecipesView: View {
    var openLists: () -> Void = {}

    @Environment(Session.self) private var session
    @State private var recipes: [RecipeSummary] = []
    @State private var tags: [String] = []
    @State private var search = ""
    @State private var filter = Filter.all
    @State private var loaded = false
    @State private var error: String?

    enum Filter: Hashable {
        case all, mark(RecipeMark), quick, tag(String)
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    chips
                    if !loaded && error == nil {
                        ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
                    } else if let error, recipes.isEmpty {
                        StateMessage(emoji: "🫠", title: "Couldn't load recipes", message: error) { Task { await load() } }
                    } else if shown.isEmpty {
                        StateMessage(emoji: emptyEmoji, title: emptyTitle, message: emptyMessage)
                    } else {
                        LazyVGrid(columns: [GridItem(.flexible(), spacing: 16), GridItem(.flexible(), spacing: 16)], spacing: 18) {
                            ForEach(shown, id: \.id) { r in
                                NavigationLink(value: r.id) {
                                    RecipeCard(recipe: r) { mark, on in toggle(mark, on, r) }
                                }
                                .buttonStyle(.plain)
                            }
                        }
                    }
                }
                .padding(.horizontal, 20)
                .padding(.bottom, 24)
            }
            .refreshable { await load() }
            .pageBackground()
            .navigationTitle("Recipes")
            .searchable(text: $search, prompt: "Search recipes or ingredients")
            .navigationDestination(for: String.self) { id in
                RecipeDetailView(recipeId: id, openLists: openLists, onMarksChanged: { marks in
                    if let i = recipes.firstIndex(where: { $0.id == id }) { recipes[i].marks = marks }
                })
            }
        }
        .task(id: QueryKey(search: search, filter: filter)) {
            // Debounce typing; the first load runs straight away.
            if loaded { try? await Task.sleep(for: .milliseconds(250)) }
            if Task.isCancelled { return }
            await load()
        }
        .task { tags = (try? await session.api.tags()) ?? [] }
    }

    private struct QueryKey: Equatable {
        let search: String
        let filter: Filter
    }

    /// "Under 30 min" is filtered here; everything else is a server query.
    private var shown: [RecipeSummary] {
        guard filter == .quick else { return recipes }
        return recipes.filter { $0.totalMinutes > 0 && $0.totalMinutes <= 30 }
    }

    private var chips: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                chip("All", nil, .all, Palette.surface)
                chip("Faves", "heart.fill", .mark(.favourite), Palette.rose)
                chip("Want to make", "bookmark.fill", .mark(.want_to_make), Palette.lilac)
                chip("Under 30 min", "timer", .quick, Palette.mint)
                ForEach(tags, id: \.self) { t in chip(t, "tag", .tag(t), Palette.sky) }
            }
            .padding(.vertical, 4)
            .padding(.trailing, 6)
        }
        .scrollClipDisabled()
    }

    private func chip(_ label: String, _ icon: String?, _ value: Filter, _ fill: Color) -> some View {
        let on = filter == value
        return Button { filter = on && value != .all ? .all : value } label: {
            Chip(label: label, systemImage: icon, fill: on ? fill : Palette.surface, selected: false)
                .rotationEffect(.degrees(on ? -2 : 0))
        }
        .accessibilityAddTraits(on ? .isSelected : [])
    }

    private var emptyEmoji: String {
        switch filter {
        case .mark(.favourite): "💖"
        case .mark(.want_to_make): "🔖"
        default: search.isEmpty ? "📖" : "🔍"
        }
    }

    private var emptyTitle: String {
        switch filter {
        case .mark(.favourite): "No faves yet"
        case .mark(.want_to_make): "Nothing saved for later"
        default: search.isEmpty ? "The recipe box is empty" : "No matches"
        }
    }

    private var emptyMessage: String {
        switch filter {
        case .mark(.favourite): "Tap the heart on a recipe you love."
        case .mark(.want_to_make): "Bookmark recipes you'd like to try."
        default: search.isEmpty ? "Add recipes on the web and they'll show up here." : "Try a different word or ingredient."
        }
    }

    private func load() async {
        let q = search.trimmingCharacters(in: .whitespaces)
        var mark: RecipeMark?
        var tag: String?
        switch filter {
        case let .mark(m): mark = m
        case let .tag(t): tag = t
        default: break
        }
        do {
            recipes = try await session.api.recipes(query: q.isEmpty ? nil : q, tag: tag, mark: mark)
            error = nil
        } catch {
            self.error = APIError.from(error).message
        }
        loaded = true
    }

    private func toggle(_ mark: RecipeMark, _ on: Bool, _ r: RecipeSummary) {
        guard let i = recipes.firstIndex(where: { $0.id == r.id }) else { return }
        let before = recipes[i].marks
        recipes[i].marks = on ? before + [mark] : before.filter { $0 != mark }
        Task {
            do {
                try await session.api.setMark(mark, on: r.id, on)
            } catch {
                if let j = recipes.firstIndex(where: { $0.id == r.id }) { recipes[j].marks = before }
            }
        }
    }
}

struct RecipeCard: View {
    let recipe: RecipeSummary
    var onMark: (RecipeMark, Bool) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            ZStack(alignment: .topTrailing) {
                RemoteImage(path: recipe.coverUrl) { RecipeArt(title: recipe.title) }
                    .frame(height: 120)
                    .frame(maxWidth: .infinity)
                    .clipped()
                let fave = recipe.has(.favourite)
                Button { onMark(.favourite, !fave) } label: {
                    Image(systemName: fave ? "heart.fill" : "heart")
                        .font(.system(size: 15, weight: .black))
                        .foregroundStyle(fave ? Color(hex: "#C42A63") : Palette.stickerInk)
                        .frame(width: 36, height: 36)
                        .background(Circle().fill(.white))
                        .overlay(Circle().strokeBorder(Palette.outline, lineWidth: 2))
                }
                .padding(8)
                .accessibilityLabel(fave ? "Remove from faves" : "Add to faves")
            }
            .overlay(alignment: .bottom) { Rectangle().fill(Palette.outline).frame(height: 2.5) }

            VStack(alignment: .leading, spacing: 6) {
                Text(recipe.title)
                    .font(.system(.headline, design: .rounded, weight: .heavy))
                    .foregroundStyle(Palette.ink)
                    .lineLimit(2)
                    .multilineTextAlignment(.leading)
                HStack(spacing: 8) {
                    if recipe.totalMinutes > 0 {
                        Label(Format.minutes(recipe.totalMinutes), systemImage: "clock")
                    }
                    if recipe.has(.want_to_make) {
                        Image(systemName: "bookmark.fill").accessibilityLabel("Want to make")
                    }
                }
                .font(.caption.weight(.bold))
                .foregroundStyle(Palette.inkMuted)
            }
            .padding(12)
            .frame(maxWidth: .infinity, minHeight: 78, alignment: .topLeading)
        }
        .clipShape(RoundedRectangle(cornerRadius: 20, style: .continuous))
        .sticker(radius: 20, shadow: 4)
        .accessibilityElement(children: .contain)
    }
}
