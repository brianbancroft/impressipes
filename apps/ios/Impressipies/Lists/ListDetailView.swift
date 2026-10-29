import SwiftUI

/// The shopping screen: quick-add pinned at the top, items grouped by aisle,
/// ticked items collapse into "In the basket".
struct ListDetailView: View {
    let listId: String
    var onChanged: () -> Void = {}

    @Environment(Session.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var store: ListStore?
    @State private var quickAdd = ""
    @State private var suggestions: [Suggestion] = []
    @State private var itemSheet: ItemSheet.Mode?
    @State private var editingList: ListFormSheet.Mode?
    @State private var basketOpen = false
    @State private var confirmClear = false
    @State private var toast: Toast?
    @FocusState private var quickAddFocused: Bool

    var body: some View {
        Group {
            if let store, let list = store.list {
                content(store, list)
            } else if let error = store?.error {
                StateMessage(emoji: "🫠", title: "Couldn't load this list", message: error) { Task { await store?.load() } }
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .pageBackground()
        .navigationBarTitleDisplayMode(.inline)
        .task {
            if store == nil {
                let s = ListStore(listId: listId, api: { [session] in session.api })
                store = s
                await s.load()
            }
        }
        .onChange(of: session.lastEvent) { _, e in
            guard e?.listId == listId else { return }
            Task { await store?.load() }
        }
        .onDisappear(perform: onChanged)
        .toast($toast)
    }

    private func content(_ store: ListStore, _ list: ShoppingList) -> some View {
        let open = store.items.filter { !$0.checked }
        let basket = store.items.filter(\.checked).sorted { $0.updatedAt > $1.updatedAt }
        return List {
            if store.isOffline {
                Label("Offline. Ticks and new items will sync when you're back.", systemImage: "icloud.slash")
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(Palette.inkMuted)
                    .frame(maxWidth: .infinity)
                    .listRowBackground(Color.clear)
                    .listRowSeparator(.hidden)
            }
            if store.items.isEmpty {
                StateMessage(emoji: list.emoji.ifEmpty("🛒"), title: "Nothing on here yet", message: "Type above and press return. \"2 cans chickpeas\" works.")
                    .listRowBackground(Color.clear)
                    .listRowSeparator(.hidden)
            } else if open.isEmpty {
                VStack(spacing: 6) {
                    Text("🎉").font(.system(size: 44))
                    Scribble(text: "nice work!")
                    Text("Everything's in the basket.").font(.headline).foregroundStyle(Palette.ink)
                }
                .frame(maxWidth: .infinity)
                .padding(.vertical, 24)
                .listRowBackground(Color.clear)
                .listRowSeparator(.hidden)
            }

            ForEach(groups(open, store), id: \.id) { group in
                Section {
                    ForEach(group.items, id: \.id) { item in row(item, store) }
                } header: {
                    AisleLabel(category: group.category)
                }
            }

            if !basket.isEmpty {
                Section {
                    if basketOpen {
                        ForEach(basket, id: \.id) { item in row(item, store) }
                        Button("Clear the basket", role: .destructive) { confirmClear = true }
                            .font(.subheadline.weight(.bold))
                            .frame(maxWidth: .infinity)
                            .listRowBackground(Color.clear)
                            .listRowSeparator(.hidden)
                    }
                } header: {
                    Button {
                        withAnimation(.snappy) { basketOpen.toggle() }
                    } label: {
                        HStack {
                            Label("In the basket · \(basket.count)", systemImage: "basket.fill")
                            Spacer()
                            Image(systemName: "chevron.down").rotationEffect(.degrees(basketOpen ? 0 : -90))
                        }
                        .font(.subheadline.weight(.heavy))
                        .foregroundStyle(Palette.inkMuted)
                        .textCase(nil)
                    }
                    .accessibilityHint(basketOpen ? "Hides ticked items" : "Shows ticked items")
                }
            }
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .environment(\.defaultMinListRowHeight, 56)
        .refreshable { await store.load() }
        .safeAreaInset(edge: .top, spacing: 0) { quickAddBar(store) }
        .navigationTitle("\(list.emoji) \(list.name)")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Menu {
                    Button("Edit list", systemImage: "pencil") { editingList = .edit(list) }
                    if !basket.isEmpty {
                        Button("Clear the basket", systemImage: "basket", role: .destructive) { confirmClear = true }
                    }
                } label: {
                    Image(systemName: "ellipsis.circle")
                }
                .accessibilityLabel("List options")
            }
        }
        .sheet(item: $itemSheet) { mode in
            ItemSheet(mode: mode, store: store)
        }
        .sheet(item: $editingList) { mode in
            ListFormSheet(mode: mode) { saved in
                if saved == nil { dismiss() } else { Task { await store.load() } }
            }
        }
        .confirmationDialog("Clear \(basket.count) ticked item\(basket.count == 1 ? "" : "s")?", isPresented: $confirmClear, titleVisibility: .visible) {
            Button("Clear the basket", role: .destructive) {
                Task {
                    let cleared = await store.clearChecked()
                    toast = Toast(message: "Cleared \(cleared.count) item\(cleared.count == 1 ? "" : "s")", actionLabel: "Undo") {
                        Task { for item in cleared { await store.restore(item) } }
                    }
                }
            }
        }
        .alert("Couldn't save that", isPresented: .init(get: { store.error != nil && store.detail != nil }, set: { if !$0 { store.dismissError() } })) {
            Button("OK", role: .cancel) {}
        } message: {
            Text(store.error ?? "")
        }
    }

    private func row(_ item: Item, _ store: ListStore) -> some View {
        ItemRow(
            item: item,
            recipeTitle: item.sourceRecipeId.flatMap { store.recipeTitles[$0] },
            pending: store.pendingIds.contains(item.id),
            onToggle: {
                UIImpactFeedbackGenerator(style: .light).impactOccurred()
                Task { await store.toggle(item) }
            }
        )
        .contentShape(Rectangle())
        .onTapGesture { itemSheet = .edit(item) }
        .listRowBackground(Color.clear)
        .listRowSeparator(.hidden)
        .listRowInsets(EdgeInsets(top: 4, leading: 16, bottom: 4, trailing: 16))
        .swipeActions(edge: .trailing) {
            Button(role: .destructive) {
                Task { await store.delete(item) }
                toast = Toast(message: "Removed \(item.name)", actionLabel: "Undo") {
                    Task { await store.restore(item) }
                }
            } label: { Label("Delete", systemImage: "trash") }
        }
        .swipeActions(edge: .leading) {
            Button { Task { await store.toggle(item) } } label: {
                Label(item.checked ? "Put back" : "Got it", systemImage: item.checked ? "arrow.uturn.backward" : "checkmark")
            }
            .tint(.green)
        }
    }

    private func quickAddBar(_ store: ListStore) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 10) {
                Image(systemName: "plus.circle.fill").foregroundStyle(Palette.accent).font(.title3)
                TextField("Add an item", text: $quickAdd)
                    .focused($quickAddFocused)
                    .submitLabel(.done)
                    .onSubmit { submitQuickAdd(store) }
                Button {
                    let parsed = QuickAdd.parse(quickAdd)
                    itemSheet = .add(name: parsed.name, quantityText: parsed.quantityText)
                    quickAdd = ""
                } label: {
                    Image(systemName: "slider.horizontal.3")
                }
                .buttonStyle(RoundIconButtonStyle(fill: Palette.butter))
                .accessibilityLabel("Add with details")
            }
            .padding(.leading, 14)
            .padding(.trailing, 4)
            .padding(.vertical, 4)
            .sticker(radius: 18, shadow: 3)

            if !suggestions.isEmpty && quickAddFocused {
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 8) {
                        ForEach(suggestions, id: \.name) { s in
                            Button {
                                Task { await addSuggestion(s, store) }
                            } label: {
                                Chip(label: s.quantityText.isEmpty ? s.name : "\(s.name) · \(s.quantityText)", systemImage: "arrow.counterclockwise", fill: Palette.mint)
                            }
                        }
                    }
                    .padding(.vertical, 2)
                    .padding(.trailing, 4)
                }
            }
        }
        .padding(.horizontal, 16)
        .padding(.top, 8)
        .padding(.bottom, 10)
        .background(Palette.bg)
        .task(id: quickAdd) {
            let q = QuickAdd.parse(quickAdd).name
            guard q.count >= 2 else { suggestions = []; return }
            try? await Task.sleep(for: .milliseconds(200))
            if Task.isCancelled { return }
            suggestions = await store.suggestions(for: q)
        }
    }

    private func submitQuickAdd(_ store: ListStore) {
        let parsed = QuickAdd.parse(quickAdd)
        guard !parsed.name.isEmpty else { return }
        quickAdd = ""
        suggestions = []
        quickAddFocused = true
        Task {
            do {
                // No category: the server picks one from the list's item memory.
                try await store.add(name: parsed.name, quantityText: parsed.quantityText, categoryId: nil)
            } catch {
                toast = Toast(message: "Couldn't add \(parsed.name)")
            }
        }
    }

    private func addSuggestion(_ s: Suggestion, _ store: ListStore) async {
        let typed = QuickAdd.parse(quickAdd)
        quickAdd = ""
        suggestions = []
        try? await store.add(name: s.name, quantityText: typed.quantityText.ifEmpty(s.quantityText), categoryId: s.categoryId)
    }

    private struct AisleGroup: Identifiable {
        let id: String
        let category: Category?
        let items: [Item]
    }

    /// Items by aisle in the list's category order, uncategorised last.
    private func groups(_ items: [Item], _ store: ListStore) -> [AisleGroup] {
        let byCat = Dictionary(grouping: items) { store.category($0.categoryId)?.id ?? "" }
        var out = store.categories.compactMap { c in
            byCat[c.id].map { AisleGroup(id: c.id, category: c, items: $0.sorted { $0.position < $1.position }) }
        }
        if let rest = byCat[""] {
            out.append(AisleGroup(id: "other", category: nil, items: rest.sorted { $0.position < $1.position }))
        }
        return out
    }
}

/// Coloured aisle label; colour is always paired with the name.
struct AisleLabel: View {
    let category: Category?

    var body: some View {
        HStack(spacing: 6) {
            Text(category?.icon.ifEmpty("🏷️") ?? "🧺")
            Text(category?.name ?? "Other")
        }
        .font(.subheadline.weight(.heavy))
        .foregroundStyle(Palette.stickerInk)
        .textCase(nil)
        .padding(.horizontal, 10)
        .padding(.vertical, 5)
        .background(Capsule().fill(Color(hex: category?.colour ?? "#7e8896").tint(0.4)))
        .overlay(Capsule().strokeBorder(Palette.outline, lineWidth: 2))
        .padding(.top, 6)
    }
}

struct ItemRow: View {
    let item: Item
    var recipeTitle: String?
    var pending = false
    let onToggle: () -> Void

    var body: some View {
        let q = Format.splitQuantity(item.quantityText)
        HStack(spacing: 12) {
            Button(action: onToggle) { TickBox(checked: item.checked) }
                .buttonStyle(.plain)
                .frame(width: 44, height: 44)
                .accessibilityLabel(item.checked ? "Ticked" : "Not ticked")
                .accessibilityHint("Toggles \(item.name)")
            QuantityBadge(qty: q.qty, unit: q.unit, muted: item.checked)
            VStack(alignment: .leading, spacing: 2) {
                Text(item.name)
                    .font(.body.weight(.semibold))
                    .strikethrough(item.checked)
                    .foregroundStyle(item.checked ? Palette.inkMuted : Palette.ink)
                if !item.note.isEmpty {
                    Text(item.note).font(.footnote).foregroundStyle(Palette.inkMuted)
                }
                if let recipeTitle {
                    HStack(spacing: 4) {
                        Image(systemName: "fork.knife")
                        Text("from \(recipeTitle)")
                    }
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(Palette.accent)
                        .lineLimit(1)
                }
            }
            Spacer(minLength: 0)
            if pending {
                // Not yet acknowledged by the server; gone on sync.
                Image(systemName: "clock")
                    .font(.footnote.weight(.semibold))
                    .foregroundStyle(Palette.inkMuted)
                    .accessibilityLabel("Waiting to sync")
            }
        }
        .padding(.vertical, 6)
        .padding(.trailing, 12)
        .padding(.leading, 4)
        .sticker(item.checked ? Palette.surface2 : Palette.surface, radius: 18, shadow: item.checked ? 0 : 3)
        .opacity(item.checked ? 0.8 : 1)
        .animation(.snappy, value: item.checked)
    }
}
