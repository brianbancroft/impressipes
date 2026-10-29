import SwiftUI

/// Make a list or change its name, emoji and colour.
struct ListFormSheet: View {
    enum Mode: Identifiable {
        case create
        case edit(ShoppingList)
        var id: String {
            if case let .edit(l) = self { return l.id }
            return "new"
        }
    }

    let mode: Mode
    var onSaved: (ShoppingList?) -> Void

    @Environment(Session.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var name = ""
    @State private var emoji = "🛒"
    @State private var colour = Palette.listColours[0]
    @State private var withAisles = true
    @State private var busy = false
    @State private var error: String?
    @State private var confirmArchive = false

    private static let emojis = ["🛒", "📦", "🥬", "🍎", "🧀", "🍞", "🥩", "🔧", "💊", "🎉", "🏕️", "🐶"]

    /// The usual supermarket aisles, the same set the seed data uses.
    private static let groceryAisles: [CategoryInput] = [
        .init(name: "Produce", colour: "#4C9A4A", icon: "🥬"),
        .init(name: "Dairy & eggs", colour: "#E8B84A", icon: "🥛"),
        .init(name: "Meat & fish", colour: "#C9594B", icon: "🥩"),
        .init(name: "Bakery", colour: "#C98A4B", icon: "🍞"),
        .init(name: "Pantry", colour: "#8A6B4E", icon: "🥫"),
        .init(name: "Frozen", colour: "#4B8FC9", icon: "🧊"),
        .init(name: "Household", colour: "#7A7F8C", icon: "🧻"),
    ]

    private var isNew: Bool { if case .create = mode { return true } else { return false } }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 22) {
                    preview

                    field("Name") {
                        TextField("Groceries", text: $name)
                            .font(.title3.weight(.semibold))
                            .submitLabel(.done)
                            .padding(.horizontal, 14)
                            .frame(minHeight: 52)
                            .sticker(radius: 16, shadow: 3)
                    }

                    field("Emoji") {
                        LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 8), count: 6), spacing: 8) {
                            ForEach(Self.emojis, id: \.self) { e in
                                Button { emoji = e } label: {
                                    Text(e)
                                        .font(.title2)
                                        .frame(maxWidth: .infinity, minHeight: 48)
                                        .sticker(e == emoji ? Palette.butter : Palette.surface, radius: 14, shadow: e == emoji ? 3 : 0)
                                }
                                .accessibilityAddTraits(e == emoji ? .isSelected : [])
                            }
                        }
                        TextField("Or type one", text: $emoji)
                            .onChange(of: emoji) { _, v in if v.count > 1 { emoji = String(v.suffix(1)) } }
                            .padding(.horizontal, 14)
                            .frame(minHeight: 44)
                            .sticker(radius: 14, shadow: 0)
                    }

                    field("Colour") {
                        HStack(spacing: 10) {
                            ForEach(Palette.listColours, id: \.self) { c in
                                Button { colour = c } label: {
                                    Circle()
                                        .fill(Color(hex: c))
                                        .frame(width: 34, height: 34)
                                        .overlay(Circle().strokeBorder(Palette.outline, lineWidth: c == colour ? 3.5 : 2))
                                        .scaleEffect(c == colour ? 1.12 : 1)
                                }
                                .accessibilityLabel("Colour \(c)")
                                .accessibilityAddTraits(c == colour ? .isSelected : [])
                            }
                        }
                    }

                    if isNew {
                        Toggle(isOn: $withAisles) {
                            VStack(alignment: .leading) {
                                Text("Start with grocery aisles").font(.headline)
                                Text("Produce, Dairy, Bakery and so on. Items are grouped by aisle.")
                                    .font(.footnote).foregroundStyle(Palette.inkMuted)
                            }
                        }
                        .tint(Palette.accent)
                    }

                    if let error {
                        Text(error).font(.callout.weight(.semibold)).foregroundStyle(Palette.danger)
                    }

                    if case .edit = mode {
                        Button("Archive this list", role: .destructive) { confirmArchive = true }
                            .font(.headline)
                            .frame(maxWidth: .infinity)
                            .padding(.top, 8)
                    }
                }
                .padding(20)
            }
            .pageBackground()
            .navigationTitle(isNew ? "New list" : "Edit list")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button(isNew ? "Create" : "Save", action: save)
                        .fontWeight(.heavy)
                        .disabled(busy || name.trimmingCharacters(in: .whitespaces).isEmpty)
                }
            }
            .confirmationDialog("Archive \(name)?", isPresented: $confirmArchive, titleVisibility: .visible) {
                Button("Archive", role: .destructive, action: archive)
            } message: {
                Text("It disappears from everyone's lists. Its items are kept.")
            }
        }
        .onAppear {
            if case let .edit(l) = mode {
                name = l.name
                emoji = l.emoji
                colour = l.colour
            }
        }
    }

    private var preview: some View {
        HStack(spacing: 14) {
            Text(emoji.ifEmpty("🛒"))
                .font(.system(size: 30))
                .frame(width: 56, height: 56)
                .background(Circle().fill(.white))
                .overlay(Circle().strokeBorder(Palette.outline, lineWidth: 2.5))
            Text(name.ifEmpty("New list"))
                .font(.display(.title2))
                .foregroundStyle(Palette.stickerInk)
            Spacer()
        }
        .padding(16)
        .sticker(Color(hex: colour).tint(0.45), radius: 22, shadow: 4)
        .accessibilityHidden(true)
    }

    private func field<Content: View>(_ label: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(label).font(.subheadline.weight(.bold)).foregroundStyle(Palette.ink)
            content()
        }
    }

    private func save() {
        let trimmed = name.trimmingCharacters(in: .whitespaces)
        busy = true
        error = nil
        Task {
            defer { busy = false }
            do {
                let saved: ShoppingList
                switch mode {
                case .create:
                    saved = try await session.api.createList(name: trimmed, emoji: emoji, colour: colour, categories: withAisles ? Self.groceryAisles : [])
                case let .edit(l):
                    saved = try await session.api.updateList(l.id, name: trimmed, emoji: emoji, colour: colour)
                }
                onSaved(saved)
                dismiss()
            } catch {
                self.error = APIError.from(error).message
            }
        }
    }

    private func archive() {
        guard case let .edit(l) = mode else { return }
        Task {
            do {
                try await session.api.archiveList(l.id)
                onSaved(nil)
                dismiss()
            } catch {
                self.error = APIError.from(error).message
            }
        }
    }
}
