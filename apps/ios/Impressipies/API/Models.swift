import Foundation

// Conveniences over the generated types. The allOf schemas (ListDetail, Recipe)
// come out of the generator as value1/value2 pairs.

extension ListDetail {
    var list: ShoppingList { value1 }
    var categories: [Category] { value2.categories.sorted { $0.sortOrder < $1.sortOrder } }
    var items: [Item] { value2.items.filter { !$0.deleted } }
    var remaining: Int { items.filter { !$0.checked }.count }
}

extension Recipe {
    var summary: RecipeSummary { value1 }
    var id: String { value1.id }
    var title: String { value1.title }
    var ingredients: [Ingredient] { value2.ingredients }
    var steps: [Step] { value2.steps }
    var notes: String { value2.notes }
    var sourceUrl: String? { value2.sourceUrl }
    var heroPath: String? { value2.images.first?.largeUrl ?? value1.coverUrl }
}

extension RecipeSummary {
    var totalMinutes: Int { (prepMinutes ?? 0) + (cookMinutes ?? 0) }
    func has(_ mark: RecipeMark) -> Bool { marks.contains(mark) }
}

enum Format {
    private static let fractions: [(Double, String)] = [
        (0.125, "⅛"), (0.25, "¼"), (1.0 / 3, "⅓"), (0.5, "½"), (2.0 / 3, "⅔"), (0.75, "¾"),
    ]

    /// 1.5 -> "1 ½", 0.33 -> "⅓", 2.04 -> "2". Same rules as the web app.
    static func quantity(_ n: Double) -> String {
        let whole = floor(n + 0.02)
        let frac = n - whole
        let w = Int(whole)
        if frac < 0.03 { return String(w) }
        if let hit = fractions.first(where: { abs($0.0 - frac) < 0.03 }) {
            return w > 0 ? "\(w) \(hit.1)" : hit.1
        }
        return String((n * 100).rounded() / 100)
    }

    static func minutes(_ m: Int) -> String {
        if m < 60 { return "\(m) min" }
        let h = m / 60
        return m % 60 == 0 ? "\(h) h" : "\(h) h \(m % 60) min"
    }

    /// Splits an ingredient for the quantity badge, scaled by factor.
    static func ingredient(_ i: Ingredient, factor: Double) -> (qty: String?, unit: String?, text: String) {
        guard let q = i.quantity else { return (nil, nil, i.rawText ?? i.item) }
        let note = i.prepNote.map { ", \($0)" } ?? ""
        return (quantity(q * factor), i.unit?.isEmpty == false ? i.unit : nil, i.item + note)
    }

    /// "2 L" -> ("2", "L"); "a few" -> ("a few", nil). An item with no quantity reads as 1.
    static func splitQuantity(_ text: String) -> (qty: String, unit: String?) {
        let t = text.trimmingCharacters(in: .whitespaces)
        if t.isEmpty { return ("1", nil) }
        if let m = t.wholeMatch(of: /(?i)x?(\d+(?:[.,\/]\d+)?)\s*(.*)/) {
            let unit = String(m.2)
            return (String(m.1), unit.isEmpty ? nil : unit)
        }
        return (t, nil)
    }
}

/// Quick-add parsing from the Add item design (IMP-4), ported from the web app:
/// name and quantity share one field ("2 cans chickpeas", "milk 2L").
enum QuickAdd {
    private static let units = "cans?|tins?|packs?|packets?|bottles?|bags?|boxes|box|bunch(?:es)?|spools?|dozen|loaf|loaves|jars?|lbs?|kg|g|ml|l|oz|cups?|tbsp|tsp"
    private static let leading = try! NSRegularExpression(pattern: "^(x?\\d+(?:[.,/]\\d+)?\\s*(?:\(units))?)\\s+(.+)$", options: .caseInsensitive)
    private static let trailing = try! NSRegularExpression(pattern: "^(.+?)\\s+(x\\d+|\\d+(?:[.,/]\\d+)?\\s*(?:\(units)))$", options: .caseInsensitive)

    static func parse(_ line: String) -> (name: String, quantityText: String) {
        let text = line.trimmingCharacters(in: .whitespaces).replacingOccurrences(of: "\\s+", with: " ", options: .regularExpression)
        let ns = text as NSString
        let range = NSRange(location: 0, length: ns.length)
        if let m = leading.firstMatch(in: text, range: range) {
            let qty = ns.substring(with: m.range(at: 1)).replacingOccurrences(of: "(\\d)\\s+", with: "$1 ", options: .regularExpression)
            return (ns.substring(with: m.range(at: 2)), qty.trimmingCharacters(in: .whitespaces))
        }
        if let m = trailing.firstMatch(in: text, range: range) {
            let qty = ns.substring(with: m.range(at: 2)).replacingOccurrences(of: "(\\d)(?=[a-zA-Z])", with: "$1 ", options: .regularExpression)
            return (ns.substring(with: m.range(at: 1)), qty)
        }
        return (text, "")
    }
}
