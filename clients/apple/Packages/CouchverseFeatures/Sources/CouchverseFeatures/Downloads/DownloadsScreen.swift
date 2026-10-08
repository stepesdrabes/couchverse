import CouchverseCore
import CouchverseDesign
import SwiftUI

#if os(iOS)
    /// The downloads on this device (plan 10.8): what the server is still preparing, what is on
    /// its way, what plays without a connection, and the room it takes. While the server is out
    /// of reach it stands in for the whole app.
    struct DownloadsScreen: View {
        /// Shown in place of the tabs, because the server is out of reach.
        var offline = false
        @Environment(CoreRuntime.self) private var core

        private var downloads: DownloadsView { core.downloads }

        var body: some View {
            List {
                if offline {
                    OfflineBanner()
                        .listRowBackground(Color.clear)
                        .listRowInsets(EdgeInsets())
                }
                switch downloads.status {
                case .idle, .loading:
                    ForEach(0..<3, id: \.self) { _ in DownloadRowSkeleton() }
                default:
                    if downloads.items.isEmpty {
                        CatalogMessage(
                            systemImage: "arrow.down.circle", title: L10n.downloadsEmpty,
                            message: L10n.downloadsEmptyHint
                        )
                        .listRowBackground(Color.clear)
                    } else {
                        Section {
                            ForEach(downloads.items, id: \.id) { item in
                                DownloadRow(item: item, offline: offline)
                            }
                        } footer: {
                            if downloads.usedBytes > 0 {
                                Text(L10n.downloadsStorage(size: DownloadLabels.size(bytes: downloads.usedBytes)))
                            }
                        }
                    }
                }
            }
            .navigationTitle(L10n.downloadsTitle)
            .coreScreen(.downloads)
        }
    }

    /// One download: its artwork, title, episode and where it stands. A finished one plays from
    /// the device with a tap, a failed one can be asked for again, and any can be removed.
    private struct DownloadRow: View {
        let item: DownloadItem
        let offline: Bool
        @Environment(CoreRuntime.self) private var core
        @Environment(\.dynamicTypeSize) private var dynamicTypeSize

        private var ref: DownloadRef { DownloadRef(id: item.id) }

        var body: some View {
            HStack(spacing: Tokens.Spacing.md) {
                if item.state == .ready {
                    Button(action: play) { details }
                        .buttonStyle(.plain)
                        .accessibilityHint(L10n.commonPlay)
                } else {
                    details
                }
                trailing
            }
            .padding(.vertical, Tokens.Spacing.xs)
            .swipeActions {
                Button(L10n.downloadRemove, systemImage: "trash", role: .destructive, action: remove)
            }
            .contextMenu {
                if item.state == .ready {
                    Button(L10n.commonPlay, systemImage: "play.fill", action: play)
                }
                if item.state == .failed {
                    Button(L10n.downloadRetry, systemImage: "arrow.clockwise", action: retry)
                }
                Button(L10n.downloadRemove, systemImage: "trash", role: .destructive, action: remove)
            }
        }

        private var details: some View {
            let layout =
                dynamicTypeSize.isAccessibilitySize
                ? AnyLayout(VStackLayout(alignment: .leading, spacing: Tokens.Spacing.sm))
                : AnyLayout(HStackLayout(spacing: Tokens.Spacing.md))
            return layout {
                ArtworkImage(image: artwork)
                    .frame(width: 112, height: 63)
                    .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
                VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                    if let episode = item.episode {
                        Text(CatalogLabels.episode(episode))
                            .typeRole(Tokens.TypeRamp.caption)
                            .foregroundStyle(Tokens.Palette.mutedText)
                    }
                    Text(DownloadLabels.name(item))
                        .typeRole(Tokens.TypeRamp.card)
                        .foregroundStyle(Tokens.Palette.text)
                        .lineLimit(2)
                    Text(DownloadLabels.state(item))
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(item.state == .failed ? Tokens.Palette.danger : Tokens.Palette.mutedText)
                        .monospacedDigit()
                }
                .multilineTextAlignment(.leading)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .contentShape(Rectangle())
            .accessibilityElement(children: .combine)
        }

        @ViewBuilder private var trailing: some View {
            switch item.state {
            case .ready:
                Image(systemName: "play.circle.fill")
                    .font(.title2)
                    .foregroundStyle(.tint)
                    .accessibilityHidden(true)
            case .failed:
                Button(action: retry) {
                    Image(systemName: "arrow.clockwise")
                        .font(.title3)
                        .frame(minWidth: 44, minHeight: 44)
                }
                .buttonStyle(.borderless)
                .accessibilityLabel(L10n.downloadRetry)
            case .queued, .preparing, .fetching:
                DownloadRing(item: item)
            }
        }

        /// The artwork kept beside the download, which shows without a network; else the
        /// server's while it can be reached.
        private var artwork: Artwork? {
            let files = DownloadFiles(directory: PlayerController.downloadsDirectory)
            if let name = item.artwork, files.size(of: name) != nil, let file = files.url(for: name) {
                return Artwork(url: file.absoluteString, accent: item.image?.accent)
            }
            return offline ? Artwork(url: "", accent: item.image?.accent) : item.image
        }

        private func play() {
            core.send(.downloadPlayRequested(ref))
        }

        private func retry() {
            core.send(.downloadRetried(ref))
        }

        private func remove() {
            core.send(.downloadRemoved(ref))
        }
    }

    /// A download on its way: a ring filling with its progress, in the accent while it comes
    /// down, dashed while the server has yet to start on it.
    struct DownloadRing: View {
        let item: DownloadItem
        @Environment(\.accent) private var accent
        @ScaledMetric(relativeTo: .body) private var size: CGFloat = 22

        var body: some View {
            ZStack {
                if item.state == .queued {
                    Circle().stroke(Tokens.Palette.mutedText, style: StrokeStyle(lineWidth: 2, dash: [3, 3]))
                } else {
                    Circle().stroke(Tokens.Palette.edgeLine, lineWidth: 2.5)
                    Circle()
                        .trim(from: 0, to: max(item.progress, 0.03))
                        .stroke(
                            item.state == .fetching ? accent.ink : Tokens.Palette.mutedText,
                            style: StrokeStyle(lineWidth: 2.5, lineCap: .round)
                        )
                        .rotationEffect(.degrees(-90))
                }
            }
            .frame(width: size, height: size)
            .animation(Tokens.Motion.standard, value: item.progress)
            .accessibilityHidden(true)
        }
    }

    /// Why only the downloads are showing.
    private struct OfflineBanner: View {
        var body: some View {
            Label(L10n.downloadsOfflineBanner, systemImage: "wifi.slash")
                .typeRole(Tokens.TypeRamp.body)
                .foregroundStyle(Tokens.Palette.text)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(Tokens.Spacing.lg)
                .background(
                    Tokens.Palette.surface2,
                    in: RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous)
                )
                .accessibilityElement(children: .combine)
        }
    }

    /// A row's shape while the device's list is read.
    private struct DownloadRowSkeleton: View {
        var body: some View {
            HStack(spacing: Tokens.Spacing.md) {
                Skeleton(width: 112, height: 63, cornerRadius: 8)
                VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                    Skeleton(width: 180, height: 16)
                    Skeleton(width: 120, height: 12)
                }
            }
            .padding(.vertical, Tokens.Spacing.xs)
        }
    }
#endif
