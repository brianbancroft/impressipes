import SwiftUI

// IMP-20: the watch is always dark, so only the dark IMP-14 values are here.

extension Color {
    init(hex: String) {
        var s = hex.trimmingCharacters(in: .whitespaces)
        if s.hasPrefix("#") { s.removeFirst() }
        if s.count == 3 { s = s.map { "\($0)\($0)" }.joined() }
        let v = UInt64(s, radix: 16) ?? 0xA5B0BD
        self.init(red: Double((v >> 16) & 0xFF) / 255, green: Double((v >> 8) & 0xFF) / 255, blue: Double(v & 0xFF) / 255)
    }
}

enum WatchPalette {
    static let bg = Color(hex: "#1A1512")
    static let surface = Color(hex: "#2A221D")
    static let surface2 = Color(hex: "#342B24")
    static let ink = Color(hex: "#F7EEE4")
    static let inkMuted = Color(hex: "#BDAE9F")
    static let accent = Color(hex: "#FF8DA5")
    static let accentSoft = Color(hex: "#4A2230")
    static let success = Color(hex: "#5BD39A")
    static let slate = Color(hex: "#A5B0BD")

    /// The list and category colours are stored as their light values; the
    /// watch draws the dark one where IMP-14 has a pair.
    private static let darkPairs: [String: String] = [
        "#c62f55": "#FF8DA5", "#b83a2b": "#FF7A66", "#e0701a": "#FFA24F", "#8a6a00": "#CFA52E",
        "#5e9626": "#9BD65F", "#0e7c86": "#4FD0D6", "#3c8de0": "#7DB8FF", "#8e4cc0": "#C79BF2",
        "#c42a7b": "#FF7DB8", "#3d3fa8": "#8F94F5", "#7a4a2e": "#B58059", "#1f6b45": "#5BD39A",
        "#7e8896": "#A5B0BD",
    ]

    static func colour(_ hex: String?) -> Color {
        guard let hex, !hex.isEmpty else { return slate }
        return Color(hex: darkPairs[hex.lowercased()] ?? hex)
    }
}

extension Font {
    /// Titles, list names and headers: SF Pro Rounded, heavy.
    static func display(_ size: CGFloat) -> Font {
        .system(size: size, weight: .heavy, design: .rounded)
    }
}
