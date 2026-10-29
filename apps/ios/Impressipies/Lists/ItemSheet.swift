import SwiftUI

/// Add an item with details, or change one: name, quantity, aisle and note.
struct ItemSheet: View {
    enum Mode: Identifiable {
        case add(name: String, quantityText: String)
        case edit(Item)
        var id: String {
            if case let .edit(i) = self { return i.id }
            return "new"
        }
    }

    let mode: Mode
    let store: ListStore

    @Environment(\.dismiss) private var dismiss
    @State private var name = ""
    @State private var amount = ""
    @State private var unit = ""
    @State private var note = ""
    @State private var categoryId: String?
    @State private var busy = false
    @State private var error: String?
    @State private var confirmDelete = false

    private static let units = ["", "can", "pack", "g", "kg", "ml", "L", "bunch", "bag", "bottle"]

    private var editing: Item? { if case let .edit(i) = mode { return i } else { return nil } }

    /// amount + unit, e.g. "2 cans"; empty means 1 of it.
    private var quantityText: String {
        let a = amount.trimmingCharacters(in: .whitespaces)
        guard !a.isEmpty else { return "" }
        guard !unit.isEmpty else { return a }
        let plural = Double(a.replacingOccurrences(of: ",", with: ".")).map { $0 != 1 } ?? true
        let short = ["g", "kg", "ml", "L"].contains(unit)
        return "\(a) \(unit)\(plural && !short ? (unit == "bunch" ? "es" : "s") : "")"
    }

    private var confirmLabel: String {
        let n = name.trimmingCharacters(in: .whitespaces)
        if editing != nil { return "Save" }
        return n.isEmpty ? "Add" : "Add \(quantityText.isEmpty ? "" : quantityText + " ")\(n)"
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 22) {
                    field("Item") {
                        TextField("Chickpeas", text: $name)
                            .font(.title3.weight(.semibold))
                            .padding(.horizontal, 14)
                            .frame(minHeight: 52)
                            .sticker(radius: 16, shadow: 3)
                    }

                    field("How many") {
                        HStack(spacing: 10) {
                            Button { step(-1) } label: { Image(systemName: "minus") }
                                .buttonStyle(RoundIconButtonStyle(fill: Palette.rose))
                                .accessibilityLabel("Fewer")
                            TextField("1", text: $amount)
                                .keyboardType(.decimalPad)
                                .multilineTextAlignment(.center)
                                .font(.system(.title2, design: .rounded, weight: .heavy))
                                .frame(minHeight: 52)
                                .sticker(Palette.butter, radius: 14, shadow: 3)
                                .foregroundStyle(Palette.stickerInk)
                            Button { step(1) } label: { Image(systemName: "plus") }
                                .buttonStyle(RoundIconButtonStyle(fill: Palette.mint))
                                .accessibilityLabel("More")
                        }
                        ScrollView(.horizontal, showsIndicators: false) {
                            HStack(spacing: 8) {
                                ForEach(Self.units, id: \.self) { u in
                                    Button { unit = u } label: { Chip(label: u.ifEmpty("each"), selected: unit == u) }
                                        .accessibilityAddTraits(unit == u ? .isSelected : [])
                                }
                            }
                            .padding(.vertical, 3)
                            .padding(.trailing, 4)
                        }
                    }

                    if !store.categories.isEmpty {
                        field("Aisle") {
                            FlowLayout(spacing: 8) {
                                ForEach(store.categories, id: \.id) { c in
                                    Button { categoryId = c.id } label: {
                                        Chip(label: "\(c.icon) \(c.name)", fill: categoryId == c.id ? Color(hex: c.colour).tint(0.45) : Palette.surface, selected: false)
                                    }
                                    .accessibilityAddTraits(categoryId == c.id ? .isSelected : [])
                                }
                                Button { categoryId = nil } label: {
                                    Chip(label: editing == nil ? "✨ Guess" : "🧺 Other", fill: categoryId == nil ? Palette.lilac : Palette.surface)
                                }
                            }
                        }
                    }

                    field("Note") {
                        TextField("Brand, size, the good kind…", text: $note, axis: .vertical)
                            .lineLimit(1...3)
                            .padding(14)
                            .sticker(radius: 16, shadow: 0)
                    }

                    field("Preview") {
                        let q = Format.splitQuantity(quantityText)
                        HStack(spacing: 12) {
                            TickBox(checked: false)
                            QuantityBadge(qty: q.qty, unit: q.unit)
                            Text(name.ifEmpty("Item")).font(.body.weight(.semibold)).foregroundStyle(Palette.ink)
                            Spacer()
                        }
                        .padding(8)
                        .sticker(radius: 18, shadow: 3)
                        .accessibilityHidden(true)
                    }

                    if let error {
                        Text(error).font(.callout.weight(.semibold)).foregroundStyle(Palette.danger)
                    }

                    Button(action: save) {
                        if busy { ProgressView().tint(Palette.onAccent) } else { Text(confirmLabel).lineLimit(1) }
                    }
                    .buttonStyle(PopButtonStyle())
                    .disabled(busy || name.trimmingCharacters(in: .whitespaces).isEmpty)

                    if editing != nil {
                        Button("Remove from list", role: .destructive) { confirmDelete = true }
                            .font(.headline)
                            .frame(maxWidth: .infinity)
                    }
                }
                .padding(20)
            }
            .scrollDismissesKeyboard(.interactively)
            .pageBackground()
            .navigationTitle(editing == nil ? "Add an item" : "Edit item")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
            }
            .confirmationDialog("Remove \(name)?", isPresented: $confirmDelete, titleVisibility: .visible) {
                Button("Remove", role: .destructive) {
                    if let item = editing { Task { await store.delete(item) } }
                    dismiss()
                }
            }
        }
        .presentationDetents([.large])
        .onAppear(perform: fill)
    }

    private func field<Content: View>(_ label: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(label).font(.subheadline.weight(.bold)).foregroundStyle(Palette.ink)
            content()
        }
    }

    private func fill() {
        let text: String
        switch mode {
        case let .add(n, q):
            name = n
            text = q
        case let .edit(item):
            name = item.name
            note = item.note
            categoryId = item.categoryId
            text = item.quantityText
        }
        let split = Format.splitQuantity(text)
        amount = text.isEmpty ? "" : split.qty
        // Map "2 cans" back onto the "can" chip; anything else stays as typed.
        if let u = split.unit {
            let base = u.hasSuffix("es") && u.lowercased().hasPrefix("bunch") ? "bunch" : (u.hasSuffix("s") && u.count > 2 ? String(u.dropLast()) : u)
            if let match = Self.units.first(where: { $0.caseInsensitiveCompare(base) == .orderedSame }) {
                unit = match
            } else {
                amount = text
            }
        }
    }

    private func step(_ delta: Double) {
        let current = Double(amount.replacingOccurrences(of: ",", with: ".")) ?? (amount.isEmpty ? 1 : 0)
        let next = max(1, current + delta)
        amount = next == next.rounded() ? String(Int(next)) : String(next)
    }

    private func save() {
        let n = name.trimmingCharacters(in: .whitespaces)
        busy = true
        error = nil
        Task {
            defer { busy = false }
            do {
                if let item = editing {
                    try await store.update(item, name: n, quantityText: quantityText, note: note, categoryId: categoryId)
                } else {
                    try await store.add(name: n, quantityText: quantityText, note: note, categoryId: categoryId)
                }
                dismiss()
            } catch {
                self.error = APIError.from(error).message
            }
        }
    }
}

/// Wraps chips onto as many lines as they need.
struct FlowLayout: Layout {
    var spacing: CGFloat = 8

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let width = proposal.width ?? .infinity
        var x: CGFloat = 0, y: CGFloat = 0, row: CGFloat = 0, widest: CGFloat = 0
        for v in subviews {
            let s = v.sizeThatFits(.unspecified)
            if x > 0, x + s.width > width {
                y += row + spacing
                x = 0
                row = 0
            }
            x += s.width + spacing
            row = max(row, s.height)
            widest = max(widest, x - spacing)
        }
        return CGSize(width: min(widest, width), height: y + row)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        var x = bounds.minX, y = bounds.minY, row: CGFloat = 0
        for v in subviews {
            let s = v.sizeThatFits(.unspecified)
            if x > bounds.minX, x + s.width > bounds.maxX {
                y += row + spacing
                x = bounds.minX
                row = 0
            }
            v.place(at: CGPoint(x: x, y: y), proposal: ProposedViewSize(s))
            x += s.width + spacing
            row = max(row, s.height)
        }
    }
}
