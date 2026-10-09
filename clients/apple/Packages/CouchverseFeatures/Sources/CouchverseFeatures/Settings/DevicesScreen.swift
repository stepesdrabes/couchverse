import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The signed-in account's devices (D12), each signed out from here except this one.
struct DevicesScreen: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.referenceDate) private var referenceDate
    @State private var revoking: DeviceCard?

    private var devices: DevicesView { core.devices }

    var body: some View {
        List {
            switch devices.status {
            case .idle, .loading:
                ForEach(0..<3, id: \.self) { _ in
                    HStack(spacing: Tokens.Spacing.lg) {
                        Skeleton(width: 36, height: 36)
                        VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                            Skeleton(width: 180, height: 16)
                            Skeleton(width: 120, height: 12)
                        }
                    }
                    .padding(.vertical, Tokens.Spacing.xs)
                    .accessibilityElement()
                    .accessibilityLabel(L10n.commonLoading)
                }
            case .failed where devices.devices.isEmpty:
                if let problem = devices.problem {
                    ProblemBanner(problem) { core.send(.devicesOpened) }
                        .listRowBackground(Color.clear)
                }
            default:
                if let problem = devices.problem {
                    ProblemBanner(problem)
                        .listRowBackground(Color.clear)
                }
                if devices.devices.isEmpty {
                    Text(L10n.devicesEmpty).foregroundStyle(Tokens.Palette.mutedText)
                }
                ForEach(devices.devices, id: \.id) { device in
                    row(device)
                        .focusable(Idiom.isTV)
                        .swipeToRemove(L10n.devicesRevoke, enabled: !device.current) { revoking = device }
                        .contextMenu {
                            if !device.current {
                                Button(
                                    L10n.devicesRevoke, systemImage: "rectangle.portrait.and.arrow.right",
                                    role: .destructive
                                ) {
                                    revoking = device
                                }
                            }
                        }
                }
            }
        }
        .overlay(alignment: .top) {
            if devices.status == .stale {
                ProgressView().padding(Tokens.Spacing.sm)
            }
        }
        .navigationTitle(L10n.devicesTitle)
        .onAppear { core.send(.devicesOpened) }
        .refreshable { core.send(.devicesOpened) }
        .confirmationDialog(
            L10n.devicesRevokeConfirmTitle(device: revoking?.name ?? ""),
            isPresented: Binding(get: { revoking != nil }, set: { if !$0 { revoking = nil } }),
            titleVisibility: .visible
        ) {
            Button(L10n.devicesRevoke, role: .destructive) {
                if let device = revoking {
                    core.send(.deviceRevoked(DeviceRef(deviceId: device.id)))
                }
            }
        } message: {
            Text(L10n.devicesRevokeConfirmMessage)
        }
    }

    private func row(_ device: DeviceCard) -> some View {
        HStack(spacing: Tokens.Spacing.lg) {
            Image(systemName: DevicePlatform.symbol(device.platform))
                .font(.title2)
                .foregroundStyle(Tokens.Palette.mutedText)
                .frame(width: 36)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                // "This device" under the name where the two do not fit on a line
                ViewThatFits(in: .horizontal) {
                    HStack(spacing: Tokens.Spacing.sm) { name(device) }
                    VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) { name(device) }
                }
                Text("\(DevicePlatform.name(device.platform)) \u{00B7} \(lastSeen(device))")
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.mutedText)
            }
        }
        .padding(.vertical, Tokens.Spacing.xs)
        .accessibilityElement(children: .combine)
    }

    @ViewBuilder private func name(_ device: DeviceCard) -> some View {
        Text(device.name)
            .typeRole(Tokens.TypeRamp.card)
            .foregroundStyle(Tokens.Palette.text)
        if device.current {
            Text(L10n.devicesThisDevice)
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.success)
        }
    }

    private func lastSeen(_ device: DeviceCard) -> String {
        guard let date = try? Date(device.lastSeenAt, strategy: .iso8601) else { return "" }
        let formatter = RelativeDateTimeFormatter()
        formatter.locale = L10n.locale
        formatter.dateTimeStyle = .named
        return L10n.devicesLastSeen(time: formatter.localizedString(for: date, relativeTo: referenceDate ?? .now))
    }
}
