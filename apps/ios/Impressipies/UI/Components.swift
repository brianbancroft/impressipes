import SwiftUI

/// Image that needs the session cookie (recipe photos), with a small in-memory cache.
struct RemoteImage<Placeholder: View>: View {
    let path: String?
    @ViewBuilder var placeholder: () -> Placeholder

    @Environment(Session.self) private var session
    @State private var image: UIImage?

    var body: some View {
        Group {
            if let image {
                Image(uiImage: image).resizable().scaledToFill()
            } else {
                placeholder()
            }
        }
        .task(id: path) {
            guard let path else { return }
            if let hit = ImageCache.shared.object(forKey: path as NSString) {
                image = hit
                return
            }
            if let data = try? await session.imageData(path), let ui = UIImage(data: data) {
                ImageCache.shared.setObject(ui, forKey: path as NSString)
                image = ui
            }
        }
    }
}

private enum ImageCache {
    static let shared = NSCache<NSString, UIImage>()
}

/// Blobby colour art for a recipe with no photo.
struct RecipeArt: View {
    let title: String

    var body: some View {
        let fill = Palette.sticker(for: title)
        ZStack {
            fill.tint(0.55)
            Circle()
                .fill(fill)
                .frame(width: 120, height: 110)
                .overlay(Circle().strokeBorder(Palette.outline, lineWidth: 2.5))
                .offset(x: 24, y: 14)
            Text(String(title.prefix(1)).uppercased())
                .font(.system(size: 44, weight: .black, design: .rounded))
                .foregroundStyle(Palette.stickerInk)
                .offset(x: 24, y: 14)
        }
        .clipped()
    }
}

/// A transient message with an optional action ("Undo", "View list").
struct Toast: Equatable {
    var message: String
    var actionLabel: String?
    var action: (() -> Void)?

    static func == (a: Toast, b: Toast) -> Bool { a.message == b.message && a.actionLabel == b.actionLabel }
}

struct ToastModifier: ViewModifier {
    @Binding var toast: Toast?

    func body(content: Content) -> some View {
        content.overlay(alignment: .bottom) {
            if let toast {
                HStack(spacing: 12) {
                    Text(toast.message)
                        .font(.subheadline.weight(.semibold))
                        .foregroundStyle(Palette.bg)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    if let label = toast.actionLabel, let action = toast.action {
                        Button(label) {
                            action()
                            self.toast = nil
                        }
                        .font(.subheadline.weight(.heavy))
                        .foregroundStyle(Palette.butter)
                    }
                }
                .padding(.horizontal, 16)
                .frame(minHeight: 52)
                .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Palette.ink))
                .padding(.horizontal, 16)
                .padding(.bottom, 12)
                .transition(.move(edge: .bottom).combined(with: .opacity))
                .task(id: toast.message) {
                    try? await Task.sleep(for: .seconds(5))
                    withAnimation { self.toast = nil }
                }
            }
        }
        .animation(.snappy, value: toast)
    }
}

extension View {
    func toast(_ toast: Binding<Toast?>) -> some View { modifier(ToastModifier(toast: toast)) }
}

/// Loading, empty and error states in the house style.
struct StateMessage: View {
    let emoji: String
    let title: String
    var message: String?
    var retry: (() -> Void)?

    var body: some View {
        VStack(spacing: 10) {
            Text(emoji).font(.system(size: 56))
            Text(title).font(.display(.title2)).foregroundStyle(Palette.ink)
            if let message {
                Text(message).font(.callout).foregroundStyle(Palette.inkMuted).multilineTextAlignment(.center)
            }
            if let retry {
                Button("Try again", action: retry)
                    .buttonStyle(PopButtonStyle(fill: Palette.butter, foreground: Palette.stickerInk, fullWidth: false))
                    .padding(.top, 6)
            }
        }
        .padding(32)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

/// − value + stepper in a sticker pill.
struct StepperPill: View {
    let label: String
    let value: String
    let decrement: () -> Void
    let increment: () -> Void

    var body: some View {
        HStack {
            Text(label).font(.headline).foregroundStyle(Palette.ink)
            Spacer()
            Button(action: decrement) { Image(systemName: "minus") }
                .buttonStyle(RoundIconButtonStyle(fill: Palette.rose))
                .accessibilityLabel("Less")
            Text(value)
                .font(.system(.title3, design: .rounded, weight: .heavy))
                .foregroundStyle(Palette.ink)
                .frame(minWidth: 44)
                .contentTransition(.numericText())
            Button(action: increment) { Image(systemName: "plus") }
                .buttonStyle(RoundIconButtonStyle(fill: Palette.mint))
                .accessibilityLabel("More")
        }
        .padding(.leading, 16)
        .padding(.trailing, 6)
        .padding(.vertical, 6)
        .sticker(radius: 20, shadow: 3)
    }
}

struct RoundIconButtonStyle: ButtonStyle {
    /// A sticker fill, so the ink glyph reads in both modes.
    var fill: Color = Palette.rose

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: 16, weight: .black))
            .foregroundStyle(Palette.stickerInk)
            .frame(width: 44, height: 44)
            .background(RoundedRectangle(cornerRadius: 14, style: .continuous).fill(fill))
            .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).strokeBorder(Palette.outline, lineWidth: 2))
            .scaleEffect(configuration.isPressed ? 0.92 : 1)
    }
}
