import SwiftUI

/// Adds a recipe's ingredients to a list, scaled to the chosen servings. Same
/// shape as the web dialog: the server merges what's already on the list and
/// guesses each new item's aisle.
struct AddToListSheet: View {
    let recipe: Recipe
    var onDone: (String) -> Void

    @Environment(Session.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var servings: Double
    @State private var lists: [ShoppingList]?
    @State private var listId: String?
    @State private var busy = false
    @State private var error: String?

    init(recipe: Recipe, servings: Double, onDone: @escaping (String) -> Void) {
        self.recipe = recipe
        self.onDone = onDone
        _servings = State(initialValue: servings)
    }

    private var factor: Double {
        guard let base = recipe.summary.servings, base > 0, servings > 0 else { return 1 }
        return servings / base
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    Text("Which list?").font(.subheadline.weight(.bold)).foregroundStyle(Palette.ink)
                    if let lists {
                        if lists.isEmpty {
                            Text("Make a list first, in the Lists tab.").foregroundStyle(Palette.inkMuted)
                        }
                        ForEach(lists, id: \.id) { l in listTile(l) }
                    } else if error == nil {
                        ProgressView().frame(maxWidth: .infinity)
                    }

                    if let base = recipe.summary.servings, base > 0 {
                        StepperPill(
                            label: "Servings (makes \(Format.quantity(base)))",
                            value: Format.quantity(servings),
                            decrement: { withAnimation { servings = max(1, servings - 1) } },
                            increment: { withAnimation { servings += 1 } }
                        )
                    }

                    VStack(alignment: .leading, spacing: 8) {
                        Text("Adding \(recipe.ingredients.count) ingredient\(recipe.ingredients.count == 1 ? "" : "s")")
                            .font(.subheadline.weight(.bold))
                            .foregroundStyle(Palette.ink)
                        ForEach(Array(recipe.ingredients.enumerated()), id: \.offset) { _, ing in
                            let parts = Format.ingredient(ing, factor: factor)
                            HStack(spacing: 10) {
                                QuantityBadge(qty: parts.qty ?? "1", unit: parts.unit)
                                Text(parts.text).font(.callout.weight(.medium)).foregroundStyle(Palette.ink)
                                Spacer(minLength: 0)
                            }
                        }
                        Text("Anything already on the list gets merged.")
                            .font(.footnote)
                            .foregroundStyle(Palette.inkMuted)
                    }
                    .padding(14)
                    .sticker(radius: 18, shadow: 0)

                    if let error {
                        Text(error).font(.callout.weight(.semibold)).foregroundStyle(Palette.danger)
                    }
                }
                .padding(20)
            }
            .safeAreaInset(edge: .bottom) {
                Button(action: add) {
                    if busy { ProgressView().tint(Palette.onAccent) } else { Label("Add to list", systemImage: "cart.badge.plus") }
                }
                .buttonStyle(PopButtonStyle())
                .disabled(busy || listId == nil)
                .padding(.horizontal, 20)
                .padding(.vertical, 10)
                .background(Palette.bg)
            }
            .pageBackground()
            .navigationTitle(recipe.title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            }
        }
        .task {
            do {
                let l = try await session.api.lists().sorted { $0.sortOrder < $1.sortOrder }
                lists = l
                listId = listId ?? l.first?.id
            } catch {
                self.error = APIError.from(error).message
                lists = []
            }
        }
    }

    private func listTile(_ l: ShoppingList) -> some View {
        let on = listId == l.id
        return Button { listId = l.id } label: {
            HStack(spacing: 12) {
                Text(l.emoji.ifEmpty("🛒"))
                    .font(.title3)
                    .frame(width: 40, height: 40)
                    .background(Circle().fill(.white))
                    .overlay(Circle().strokeBorder(Palette.outline, lineWidth: 2))
                Text(l.name).font(.display(.title3)).foregroundStyle(Palette.stickerInk)
                Spacer()
                TickBox(checked: on, colour: .white)
            }
            .padding(.horizontal, 12)
            .frame(minHeight: 60)
            .sticker(Color(hex: l.colour).tint(on ? 0.5 : 0.22), radius: 18, shadow: on ? 4 : 0)
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(on ? .isSelected : [])
    }

    private func add() {
        guard let listId else { return }
        busy = true
        error = nil
        Task {
            defer { busy = false }
            do {
                let scaled = (recipe.summary.servings ?? 0) > 0 ? servings : nil
                let res = try await session.api.addRecipe(recipe.id, to: listId, servings: scaled)
                let list = lists?.first { $0.id == listId }
                let added = res.added.count, merged = res.merged.count
                var message = "Added \(added) item\(added == 1 ? "" : "s")"
                if merged > 0 { message += ", merged \(merged)" }
                message += " to \(list?.emoji ?? "") \(list?.name ?? "the list")"
                UINotificationFeedbackGenerator().notificationOccurred(.success)
                onDone(message)
                dismiss()
            } catch {
                self.error = APIError.from(error).message
            }
        }
    }
}
