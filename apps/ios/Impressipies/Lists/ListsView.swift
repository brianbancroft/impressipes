import SwiftUI

/// All lists as big colour tiles with how much is left to get.
struct ListsView: View {
    @Environment(Session.self) private var session
    @State private var lists: [ShoppingList] = []
    @State private var details: [String: ListDetail] = [:]
    @State private var loaded = false
    @State private var error: String?
    @State private var editing: ListFormSheet.Mode?
    @State private var path = NavigationPath()

    var body: some View {
        NavigationStack(path: $path) {
            Group {
                if !loaded && error == nil {
                    ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
                } else if let error, lists.isEmpty {
                    StateMessage(emoji: "🫠", title: "Couldn't load lists", message: error) { Task { await load() } }
                } else if lists.isEmpty {
                    StateMessage(emoji: "🛒", title: "No lists yet", message: "Make one for the grocery run, Costco or the hardware store.")
                } else {
                    ScrollView {
                        LazyVStack(spacing: 18) {
                            ForEach(Array(lists.enumerated()), id: \.element.id) { index, list in
                                NavigationLink(value: list.id) {
                                    ListTile(list: list, detail: details[list.id], tilt: index.isMultiple(of: 2) ? -0.6 : 0.6)
                                }
                                .buttonStyle(.plain)
                                .contextMenu {
                                    Button("Edit list", systemImage: "pencil") { editing = .edit(list) }
                                    Button("Archive", systemImage: "archivebox", role: .destructive) { archive(list) }
                                }
                            }
                        }
                        .padding(20)
                    }
                    .refreshable { await load() }
                }
            }
            .pageBackground()
            .navigationTitle("Lists")
            .toolbar {
                ToolbarItem(placement: .primaryAction) {
                    Button { editing = .create } label: { Image(systemName: "plus") }
                        .accessibilityLabel("New list")
                }
            }
            .navigationDestination(for: String.self) { id in
                ListDetailView(listId: id, onChanged: { Task { await load() } })
            }
            .sheet(item: $editing) { mode in
                ListFormSheet(mode: mode) { saved in
                    Task {
                        await load()
                        if case .create = mode, let saved { path.append(saved.id) }
                    }
                }
            }
        }
        .task { await load() }
        .onChange(of: session.lastEvent) { _, e in
            guard let e, details[e.listId] != nil else { return }
            Task { details[e.listId] = try? await session.api.list(e.listId) }
        }
    }

    private func load() async {
        do {
            let fetched = try await session.api.lists().sorted { $0.sortOrder < $1.sortOrder }
            lists = fetched
            error = nil
            // Counts come from each list's items; there are only a handful of lists.
            let api = session.api
            await withTaskGroup(of: (String, ListDetail?).self) { group in
                for l in fetched {
                    group.addTask { (l.id, try? await api.list(l.id)) }
                }
                for await (id, d) in group {
                    if let d { details[id] = d }
                }
            }
        } catch {
            self.error = APIError.from(error).message
        }
        loaded = true
    }

    private func archive(_ list: ShoppingList) {
        Task {
            try? await session.api.archiveList(list.id)
            await load()
        }
    }
}

private struct ListTile: View {
    let list: ShoppingList
    let detail: ListDetail?
    var tilt: Double = 0

    var body: some View {
        let colour = Color(hex: list.colour)
        let total = detail?.items.count ?? 0
        let left = detail?.remaining ?? 0
        HStack(spacing: 16) {
            Text(list.emoji.ifEmpty("🛒"))
                .font(.system(size: 34))
                .frame(width: 64, height: 64)
                .background(Circle().fill(.white))
                .overlay(Circle().strokeBorder(Palette.outline, lineWidth: 2.5))
                .rotationEffect(.degrees(-tilt * 4))
            VStack(alignment: .leading, spacing: 4) {
                Text(list.name)
                    .font(.display(.title2))
                    .foregroundStyle(Palette.stickerInk)
                    .lineLimit(2)
                Text(detail == nil ? " " : left == 0 ? (total == 0 ? "Empty" : "All done!") : "\(left) to get")
                    .font(.subheadline.weight(.bold))
                    .foregroundStyle(Palette.stickerInk.opacity(0.75))
            }
            Spacer(minLength: 0)
            ProgressRing(progress: total == 0 ? 0 : Double(total - left) / Double(total))
        }
        .padding(18)
        .sticker(colour.tint(0.45), radius: 24, shadow: 5)
        .rotationEffect(.degrees(tilt))
        .accessibilityElement(children: .combine)
        .accessibilityHint("Opens the list")
    }
}

private struct ProgressRing: View {
    let progress: Double

    var body: some View {
        ZStack {
            Circle().stroke(Palette.stickerInk.opacity(0.15), lineWidth: 7)
            Circle()
                .trim(from: 0, to: progress)
                .stroke(Palette.stickerInk, style: StrokeStyle(lineWidth: 7, lineCap: .round))
                .rotationEffect(.degrees(-90))
            Text("\(Int((progress * 100).rounded()))%")
                .font(.caption2.weight(.heavy))
                .foregroundStyle(Palette.stickerInk)
        }
        .frame(width: 48, height: 48)
        .animation(.snappy, value: progress)
        .accessibilityLabel("\(Int((progress * 100).rounded())) percent got")
    }
}
