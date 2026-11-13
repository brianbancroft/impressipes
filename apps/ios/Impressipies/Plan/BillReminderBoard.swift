import SwiftUI

/// One-column bill reminder board for the bottom of Home.
///
/// Pass `reminders` once a source exists. The list area is the seam: empty today,
/// a single column of rows when it isn't.
///
/// TODO: The data source is still to be decided.
struct BillReminderBoard: View {
    var reminders: [BillReminder] = []

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Bills")
                .font(.display(.title3))
                .foregroundStyle(Palette.ink)
                .accessibilityAddTraits(.isHeader)
            list
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .sticker(radius: 18, shadow: 3)
    }

    private var list: some View {
        Group {
            if reminders.isEmpty {
                Text("Nothing due")
                    .font(.body.weight(.semibold))
                    .foregroundStyle(Palette.inkMuted)
                    .frame(maxWidth: .infinity, minHeight: 88)
                    .accessibilityLabel("Nothing due")
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: 0) {
                        ForEach(Array(reminders.enumerated()), id: \.element.id) { index, reminder in
                            if index > 0 {
                                Rectangle().fill(Palette.line).frame(height: 1)
                            }
                            BillReminderRow(reminder: reminder)
                        }
                    }
                }
                .scrollBounceBehavior(.basedOnSize)
                // Hug the rows, and scroll once the column gets long.
                .frame(maxHeight: 180)
                .fixedSize(horizontal: false, vertical: true)
            }
        }
        .frame(maxWidth: .infinity, alignment: .top)
        .background(
            RoundedRectangle(cornerRadius: 12, style: .continuous)
                .fill(Palette.bg)
        )
    }
}

/// One row on the bill board. Presentation only, until a data source is chosen.
struct BillReminder: Identifiable, Hashable {
    var id: String
    var title: String
    /// Short secondary line, such as when it's due. The source decides the wording.
    var detail: String = ""
}

private struct BillReminderRow: View {
    let reminder: BillReminder

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(reminder.title)
                .font(.body.weight(.semibold))
                .foregroundStyle(Palette.ink)
                .lineLimit(1)
            if !reminder.detail.isEmpty {
                Text(reminder.detail)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(Palette.inkMuted)
                    .lineLimit(1)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 12)
        .padding(.vertical, 10)
        .accessibilityElement(children: .combine)
    }
}
