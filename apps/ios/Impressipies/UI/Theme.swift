import SwiftUI
import UIKit

// "Sticker-book kitchen" tokens from the bright & quirky redesign, matching the
// web app's styles.css. Ink outlines and solid offset shadows, no blur.

extension Color {
    init(hex: String) {
        var s = hex.trimmingCharacters(in: .whitespaces)
        if s.hasPrefix("#") { s.removeFirst() }
        if s.count == 3 { s = s.map { "\($0)\($0)" }.joined() }
        let v = UInt64(s, radix: 16) ?? 0x7E8896
        self.init(red: Double((v >> 16) & 0xFF) / 255, green: Double((v >> 8) & 0xFF) / 255, blue: Double(v & 0xFF) / 255)
    }

    fileprivate init(light: UInt32, dark: UInt32) {
        self.init(uiColor: UIColor { $0.userInterfaceStyle == .dark ? UIColor(rgb: dark) : UIColor(rgb: light) })
    }

    /// The colour mixed toward white, like the web's `tint()`. Stays light in dark mode
    /// so sticker text (always ink) keeps its contrast.
    func tint(_ amount: Double) -> Color {
        let c = UIColor(self).resolvedColor(with: .init(userInterfaceStyle: .light))
        var r: CGFloat = 0, g: CGFloat = 0, b: CGFloat = 0, a: CGFloat = 0
        c.getRed(&r, green: &g, blue: &b, alpha: &a)
        let mix = { (x: CGFloat) in Double(x) * amount + (1 - amount) }
        return Color(red: mix(r), green: mix(g), blue: mix(b))
    }
}

private extension UIColor {
    convenience init(rgb: UInt32) {
        self.init(red: CGFloat((rgb >> 16) & 0xFF) / 255, green: CGFloat((rgb >> 8) & 0xFF) / 255, blue: CGFloat(rgb & 0xFF) / 255, alpha: 1)
    }
}

enum Palette {
    static let bg = Color(light: 0xFFF4F8, dark: 0x1E1124)
    static let surface = Color(light: 0xFFFFFF, dark: 0x2D1A34)
    static let surface2 = Color(light: 0xFFE9F1, dark: 0x3A2442)
    static let line = Color(light: 0xE6D3DE, dark: 0x4D3555)
    static let outline = Color(light: 0x2A1630, dark: 0x0B040D)
    static let ink = Color(light: 0x2A1630, dark: 0xFFF4F8)
    static let inkMuted = Color(light: 0x6E5672, dark: 0xCBB3CF)
    static let accent = Color(light: 0xC42A63, dark: 0xFF7AA8)
    static let onAccent = Color(light: 0xFFFFFF, dark: 0x2A0A18)
    static let accentSoft = Color(light: 0xFFC7DA, dark: 0x4F2340)
    static let danger = Color(light: 0xB3261E, dark: 0xFF8A80)
    /// Tick box ring: the dark outline vanishes on the dark ground, so it lightens there.
    static let tickRing = Color(light: 0x2A1630, dark: 0xCBB3CF)

    /// Sticker fills always carry ink text, in both modes.
    static let stickerInk = Color(hex: "#2A1630")
    static let butter = Color(hex: "#FFE27A")
    static let mint = Color(hex: "#A6EBCF")
    static let lilac = Color(hex: "#CDBBFF")
    static let tangerine = Color(hex: "#FFB27D")
    static let sky = Color(hex: "#A9DBFF")
    static let rose = Color(hex: "#FFC7DA")
    static let stickers = [butter, mint, lilac, tangerine, sky, rose]

    /// Stable sticker colour for a string, for recipes without a photo.
    static func sticker(for key: String) -> Color {
        stickers[Int(key.unicodeScalars.reduce(0) { ($0 &* 31 &+ Int($1.value)) & 0xFFFF }) % stickers.count]
    }

    /// List colours offered when making a list (same set as the web).
    static let listColours = ["#c62f55", "#b83a2b", "#e0701a", "#8a6a00", "#5e9626", "#0e7c86", "#3c8de0", "#8e4cc0"]
    static let memberColours = ["#e11d48", "#f97316", "#eab308", "#16a34a", "#0891b2", "#2563eb", "#7c3aed", "#db2777"]
}

extension Font {
    /// Headlines: heavy and rounded, standing in for Bricolage Grotesque 800.
    static func display(_ style: Font.TextStyle = .largeTitle) -> Font {
        .system(style, design: .rounded, weight: .heavy)
    }
}

// MARK: Sticker surfaces

struct StickerBackground: ViewModifier {
    var fill: Color = Palette.surface
    var radius: CGFloat = 20
    var shadow: CGFloat = 4
    var lineWidth: CGFloat = 2.5

    func body(content: Content) -> some View {
        content
            .background(
                RoundedRectangle(cornerRadius: radius, style: .continuous)
                    .fill(fill)
                    .overlay(RoundedRectangle(cornerRadius: radius, style: .continuous).strokeBorder(Palette.outline, lineWidth: lineWidth))
                    .background(
                        RoundedRectangle(cornerRadius: radius, style: .continuous)
                            .fill(Palette.outline)
                            .offset(x: shadow, y: shadow)
                    )
            )
    }
}

extension View {
    func sticker(_ fill: Color = Palette.surface, radius: CGFloat = 20, shadow: CGFloat = 4) -> some View {
        modifier(StickerBackground(fill: fill, radius: radius, shadow: shadow))
    }

    /// The blush page ground.
    func pageBackground() -> some View {
        background(Palette.bg.ignoresSafeArea())
    }
}

/// Chunky pop button: pressing pushes it into its shadow.
struct PopButtonStyle: ButtonStyle {
    var fill: Color = Palette.accent
    var foreground: Color = Palette.onAccent
    var fullWidth = true

    func makeBody(configuration: Configuration) -> some View {
        let pressed = configuration.isPressed
        return configuration.label
            .font(.headline.weight(.heavy))
            .foregroundStyle(foreground)
            .padding(.horizontal, 18)
            .frame(maxWidth: fullWidth ? .infinity : nil, minHeight: 50)
            .background(
                RoundedRectangle(cornerRadius: 18, style: .continuous)
                    .fill(fill)
                    .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).strokeBorder(Palette.outline, lineWidth: 2.5))
            )
            .background(
                RoundedRectangle(cornerRadius: 18, style: .continuous)
                    .fill(Palette.outline)
                    .offset(x: pressed ? 0 : 4, y: pressed ? 0 : 4)
            )
            .offset(x: pressed ? 4 : 0, y: pressed ? 4 : 0)
            .animation(.snappy(duration: 0.12), value: pressed)
    }
}

/// A filter or meta chip.
struct Chip: View {
    let label: String
    var systemImage: String?
    var fill: Color = Palette.surface
    var selected = false

    var body: some View {
        HStack(spacing: 5) {
            if let systemImage { Image(systemName: systemImage).imageScale(.small) }
            Text(label)
        }
        .font(.subheadline.weight(.bold))
        .foregroundStyle(selected || fill != Palette.surface ? Palette.stickerInk : Palette.ink)
        .padding(.horizontal, 12)
        .frame(minHeight: 36)
        .background(Capsule().fill(selected ? Palette.rose : fill))
        .overlay(Capsule().strokeBorder(Palette.outline, lineWidth: 2))
        .background(Capsule().fill(Palette.outline).offset(x: selected ? 2 : 0, y: selected ? 2 : 0))
    }
}

/// The butter quantity badge: big number, small unit, fixed width so a list lines up.
struct QuantityBadge: View {
    let qty: String
    var unit: String?
    var muted = false

    var body: some View {
        VStack(spacing: -2) {
            Text(qty)
                .font(.system(.headline, design: .rounded, weight: .heavy))
            if let unit {
                Text(unit)
                    .font(.caption2.weight(.bold))
                    .textCase(.uppercase)
            }
        }
        // Grows to fit rather than clipping or shrinking the unit.
        .lineLimit(1)
        .fixedSize()
        .foregroundStyle(Palette.stickerInk)
        .padding(.horizontal, 6)
        .frame(minWidth: 54, minHeight: 44)
        .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(muted ? Palette.line : Palette.butter))
        .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).strokeBorder(Palette.outline, lineWidth: 2))
        .accessibilityElement(children: .combine)
    }
}

/// Round tick box for list items and pickers.
struct TickBox: View {
    let checked: Bool
    var colour: Color = Palette.mint

    var body: some View {
        ZStack {
            Circle().fill(checked ? colour : Palette.surface)
            Circle().strokeBorder(checked ? Palette.outline : Palette.tickRing, lineWidth: 2.5)
            if checked {
                Image(systemName: "checkmark")
                    .font(.system(size: 14, weight: .black))
                    .foregroundStyle(Palette.stickerInk)
            }
        }
        .frame(width: 30, height: 30)
    }
}

/// Small handwritten-feeling aside ("nice work!"), used sparingly.
struct Scribble: View {
    let text: String
    var body: some View {
        Text(text)
            .font(.system(.callout, design: .serif, weight: .semibold).italic())
            .foregroundStyle(Palette.accent)
            .rotationEffect(.degrees(-3))
    }
}
