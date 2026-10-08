import CouchverseCore
import CouchverseDesign
import SwiftUI

#if os(iOS)
    import PhotosUI
#endif

/// Editing your own profile (D18): the pictures, the name and bio, whether you appear on public
/// profiles and leaderboards (with rankings on), and the password, each saved on its own. A phone
/// or tablet picks pictures from the photo library; a TV has none, so it shows a QR code that
/// opens the profile on a phone (plan 10.6) and edits the text with the remote.
struct ProfileEditorScreen: View {
    /// A save whose outcome the screen shows: only the ones asked for on this visit, so an earlier
    /// visit's "Password changed" never greets the next.
    enum Part: Hashable {
        case details
        case password
        case avatar
        case banner
    }

    @Environment(CoreRuntime.self) private var core
    @State private var asked: Set<Part>
    @State private var name = ""
    @State private var bio = ""
    @State private var seeded = false
    @State private var current = ""
    @State private var new = ""
    @State private var confirm = ""
    @FocusState private var nameFocused: Bool

    /// `showing` are saves already asked for, whose outcome previews and snapshots show.
    init(showing: Set<Part> = []) {
        _asked = State(initialValue: showing)
    }

    static let bioLimit = 2000
    static let passwordMinimum = 8

    private var user: SessionUser? { core.session.user }
    private var editor: ProfileEditorView { core.profileEditor }
    private var rankings: Bool { core.session.features.rankings }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Idiom.isTV ? 48 : Tokens.Spacing.xxl) {
                #if os(iOS)
                    PicturesSection(asked: $asked)
                #else
                    PicturesHandOff()
                #endif
                details
                if rankings {
                    visibility
                }
                password
            }
            .padding(.horizontal, CardMetrics.edge)
            .padding(.vertical, Idiom.isTV ? 40 : Tokens.Spacing.xl)
            .readableWidth(Idiom.isTV ? 1100 : 600)
        }
        .scrollBounceBehavior(.basedOnSize)
        .background(Tokens.Palette.bg)
        .navigationTitle(L10n.profileHeading)
        .navigationBarTitleDisplayModeInline()
        .defaultFocus($nameFocused, Idiom.isTV)
        .opening(rankings ? user.map { Surface.profile($0.username) } : nil)
        .onChange(of: user?.username, initial: true) { seed() }
        .onChange(of: editor.password.status) { _, status in
            if status == .loaded && asked.contains(.password) {
                current = ""
                new = ""
                confirm = ""
            }
        }
    }

    private var details: some View {
        let saving = editor.details.status == .loading && asked.contains(.details)
        return ReadingBlock(title: nil, focusable: false) {
            FormField(L10n.profileDisplayName, text: $name, kind: .name)
                .focused($nameFocused)
            BioField(text: $bio, limit: Self.bioLimit)
            Button(action: saveDetails) {
                ActionLabel(L10n.commonSave, busy: saving)
            }
            .primaryAction()
            .disabled(!detailsDirty || name.trimmingCharacters(in: .whitespaces).isEmpty || saving)
            .accessibilityIdentifier("save-profile")
            if asked.contains(.details) {
                SaveOutcome(state: editor.details, saved: L10n.profileSaved, failed: L10n.profileSaveFailed)
            }
        }
        .tvFocusSection()
    }

    private var detailsDirty: Bool {
        guard let user else { return false }
        return name.trimmingCharacters(in: .whitespaces) != user.displayName || bio != user.bio
    }

    private func seed() {
        guard !seeded, let user else { return }
        seeded = true
        name = user.displayName
        bio = user.bio
    }

    private func saveDetails() {
        guard detailsDirty else { return }
        asked.insert(.details)
        core.send(.profileEditSubmitted(ProfileEdit(displayName: name.trimmingCharacters(in: .whitespaces), bio: bio)))
    }

    private var visibility: some View {
        ReadingBlock(title: L10n.profileTabPrivacy, focusable: false) {
            Toggle(isOn: publicProfile) {
                VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
                    Text(L10n.profilesPublicLabel)
                        .typeRole(Tokens.TypeRamp.card)
                        .foregroundStyle(Tokens.Palette.text)
                    Text(L10n.profilesPublicHint)
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.muted)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
            .accessibilityIdentifier("public-profile-toggle")
        }
        .tvFocusSection()
    }

    /// Your profile as the core has it: flipped at once, flipped back (with a notice) when the
    /// server refuses.
    private var publicProfile: Binding<Bool> {
        Binding(
            get: { user.flatMap { core.profile($0.username).profile?.public } ?? true },
            set: { core.send(.profileVisibilityChanged(PublicChoice(public: $0))) })
    }

    private var password: some View {
        ReadingBlock(title: L10n.profilePasswordHeading, focusable: false) {
            FormField(L10n.profileCurrentPassword, text: $current, kind: .password)
            FormField(L10n.profileNewPassword, text: $new, kind: .password)
            FormField(L10n.profileConfirmPassword, text: $confirm, kind: .password)
            if let problem = Self.passwordProblem(new: new, confirm: confirm) {
                Text(problem)
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.danger)
            }
            Button(action: changePassword) {
                ActionLabel(
                    L10n.profilePasswordHeading, busy: editor.password.status == .loading && asked.contains(.password))
            }
            .secondaryAction()
            .disabled(!canChangePassword)
            .accessibilityIdentifier("change-password")
            if asked.contains(.password) {
                SaveOutcome(
                    state: editor.password, saved: L10n.profilePasswordChanged,
                    failed: L10n.profilePasswordChangeFailed)
            }
        }
        .tvFocusSection()
    }

    /// What is wrong with the new password as typed, once there is something to judge.
    static func passwordProblem(new: String, confirm: String) -> String? {
        if !new.isEmpty && new.count < passwordMinimum {
            return L10n.profilePasswordTooShort(count: passwordMinimum)
        }
        if !confirm.isEmpty && confirm != new {
            return L10n.profilePasswordMismatch
        }
        return nil
    }

    private var canChangePassword: Bool {
        !current.isEmpty && new.count >= Self.passwordMinimum && confirm == new
            && editor.password.status != .loading
    }

    private func changePassword() {
        guard canChangePassword else { return }
        asked.insert(.password)
        core.send(.passwordChangeSubmitted(PasswordForm(current: current, new: new)))
    }
}

/// The bio as Markdown, up to the server's limit.
private struct BioField: View {
    @Binding var text: String
    let limit: Int

    var body: some View {
        let field = TextField(
            L10n.profileBio, text: $text,
            prompt: Text(L10n.profileBioPlaceholder).foregroundStyle(Tokens.Palette.faint), axis: .vertical
        )
        .lineLimit(4...12)
        .foregroundStyle(Tokens.Palette.text)
        .onChange(of: text) { _, typed in
            if typed.count > limit {
                text = String(typed.prefix(limit))
            }
        }
        #if os(tvOS)
            field.accessibilityLabel(L10n.profileBio)
        #else
            VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
                Text(L10n.profileBio)
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.muted)
                field
                    .padding(Tokens.Spacing.md)
                    .background(
                        Tokens.Palette.surface2,
                        in: RoundedRectangle(cornerRadius: Tokens.Radius.input, style: .continuous)
                    )
                    .overlay {
                        RoundedRectangle(cornerRadius: Tokens.Radius.input, style: .continuous)
                            .strokeBorder(Tokens.Palette.edge)
                    }
                    .accessibilityLabel(L10n.profileBio)
                if text.count > limit - 200 {
                    Text("\(text.count) / \(limit)")
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.faint)
                        .monospacedDigit()
                        .frame(maxWidth: .infinity, alignment: .trailing)
                }
            }
        #endif
    }
}

/// How a save of this visit went: the success line, or what went wrong. While it runs its button
/// says so.
private struct SaveOutcome: View {
    let state: SaveState
    let saved: String
    let failed: String

    var body: some View {
        switch state.status {
        case .loaded:
            Label(saved, systemImage: "checkmark.circle.fill")
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.success)
        case .failed, .notFound:
            Label(RanksWords.failure(state.problem, fallback: failed), systemImage: "exclamationmark.triangle.fill")
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.danger)
                .fixedSize(horizontal: false, vertical: true)
        default:
            EmptyView()
        }
    }
}

#if os(iOS)
    /// The banner and the avatar, picked from the photo library and uploaded as they are chosen.
    private struct PicturesSection: View {
        @Binding var asked: Set<ProfileEditorScreen.Part>

        @Environment(CoreRuntime.self) private var core
        @State private var avatarItem: PhotosPickerItem?
        @State private var bannerItem: PhotosPickerItem?
        /// Picked photos being made ready for the upload.
        @State private var preparing: Set<ImageSlot> = []
        /// Picked photos that could not be read.
        @State private var unreadable: Set<ImageSlot> = []
        /// The last change of a slot was a removal, which fails in other words.
        @State private var removing: Set<ImageSlot> = []

        private var user: SessionUser? { core.session.user }
        private var card: AccountCard? { core.accounts.accounts.first { $0.id == core.app.activeAccount } }
        /// The banner as your profile shows it; without rankings only that there is one is known.
        private var banner: Artwork? { user.flatMap { core.profile($0.username).profile?.banner } }

        var body: some View {
            ReadingBlock(title: nil, focusable: false) {
                bannerPreview
                ViewThatFits(in: .horizontal) {
                    HStack(spacing: Tokens.Spacing.md) { bannerActions }
                    VStack(alignment: .leading, spacing: Tokens.Spacing.md) { bannerActions }
                }
                Text(L10n.profileBannerHint)
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.faint)
                    .fixedSize(horizontal: false, vertical: true)
                outcome(.banner)
                HStack(spacing: Tokens.Spacing.lg) {
                    AvatarView(url: card?.avatarUrl, seed: user?.username ?? "", name: user?.displayName ?? "")
                        .frame(width: 72, height: 72)
                        .overlay { if busy(.avatar) { ProgressView() } }
                    VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                        let title = L10n.profileChangePicture
                        PhotosPicker(selection: $avatarItem, matching: .images, preferredItemEncoding: .compatible) {
                            Label(title, systemImage: "camera")
                        }
                        .secondaryAction()
                        .disabled(busy(.avatar))
                        .accessibilityIdentifier("pick-avatar")
                        if user?.avatarId != nil {
                            Button(L10n.profileRemovePicture, systemImage: "trash", role: .destructive) {
                                remove(.avatar)
                            }
                            .buttonStyle(.borderless)
                            .disabled(busy(.avatar))
                        }
                    }
                }
                outcome(.avatar)
            }
            .onChange(of: avatarItem) { _, item in pick(item, slot: .avatar) }
            .onChange(of: bannerItem) { _, item in pick(item, slot: .banner) }
        }

        private var bannerPreview: some View {
            ZStack {
                if banner != nil {
                    ArtworkImage(image: banner)
                } else {
                    Rectangle().fill(Tokens.Palette.surface2)
                    Label(L10n.profileBanner, systemImage: "photo")
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.faint)
                }
                if busy(.banner) {
                    ProgressView()
                }
            }
            .frame(height: 120)
            .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
            .accessibilityHidden(true)
        }

        @ViewBuilder private var bannerActions: some View {
            let title = user?.bannerId == nil ? L10n.profileBannerUpload : L10n.profileBannerReplace
            PhotosPicker(selection: $bannerItem, matching: .images, preferredItemEncoding: .compatible) {
                Label(title, systemImage: "photo")
            }
            .secondaryAction()
            .disabled(busy(.banner))
            .accessibilityIdentifier("pick-banner")
            if user?.bannerId != nil {
                Button(L10n.profileBannerRemove, systemImage: "trash", role: .destructive) { remove(.banner) }
                    .buttonStyle(.borderless)
                    .disabled(busy(.banner))
            }
        }

        @ViewBuilder private func outcome(_ slot: ImageSlot) -> some View {
            let part: ProfileEditorScreen.Part = slot == .avatar ? .avatar : .banner
            let state = slot == .avatar ? core.profileEditor.avatar : core.profileEditor.banner
            if unreadable.contains(slot) {
                SaveOutcome(state: SaveState(status: .failed), saved: "", failed: uploadFailed(slot))
            } else if asked.contains(part) && !preparing.contains(slot) {
                SaveOutcome(
                    state: state, saved: slot == .avatar ? L10n.profilePictureUpdated : L10n.commonSaved,
                    failed: removing.contains(slot) ? removeFailed(slot) : uploadFailed(slot))
            }
        }

        private func uploadFailed(_ slot: ImageSlot) -> String {
            slot == .avatar ? L10n.profileAvatarUploadFailed : L10n.profileBannerUploadFailed
        }

        private func removeFailed(_ slot: ImageSlot) -> String {
            slot == .avatar ? L10n.profileAvatarRemoveFailed : L10n.commonDeleteFailed
        }

        private func busy(_ slot: ImageSlot) -> Bool {
            let state = slot == .avatar ? core.profileEditor.avatar : core.profileEditor.banner
            return preparing.contains(slot) || state.status == .loading
        }

        /// The photo goes to the core as a handle; the upload effect sends the file.
        private func pick(_ item: PhotosPickerItem?, slot: ImageSlot) {
            guard let item else { return }
            asked.insert(slot == .avatar ? .avatar : .banner)
            removing.remove(slot)
            unreadable.remove(slot)
            preparing.insert(slot)
            Task {
                do {
                    guard let data = try await item.loadTransferable(type: Data.self) else {
                        throw UploadFileError.notAnImage
                    }
                    // an avatar is shown small; a banner spans a TV
                    let handle = try await UploadFiles().holdImage(data, maxPixels: slot == .avatar ? 1024 : 2560)
                    core.send(.imageChosen(ImageChoice(slot: slot, file: handle)))
                } catch {
                    unreadable.insert(slot)
                }
                preparing.remove(slot)
                // so picking the same photo again is a change
                if slot == .avatar {
                    avatarItem = nil
                } else {
                    bannerItem = nil
                }
            }
        }

        private func remove(_ slot: ImageSlot) {
            asked.insert(slot == .avatar ? .avatar : .banner)
            removing.insert(slot)
            unreadable.remove(slot)
            core.send(.imageRemoved(ImageSlotRef(slot: slot)))
        }
    }
#else
    /// A TV has no photos: a QR code opens your profile on a phone (the web's, where the pictures
    /// are changed), and the current ones can still be removed from here.
    private struct PicturesHandOff: View {
        @Environment(CoreRuntime.self) private var core
        @ScaledMetric(relativeTo: .body) private var qrSize: CGFloat = 280

        private var user: SessionUser? { core.session.user }

        /// The web profile on the account's server.
        private var profileURL: String? {
            let serverId = core.accounts.accounts.first { $0.id == core.app.activeAccount }?.serverId
            return core.servers.servers.first { $0.id == serverId }.map { "\($0.url)/profile" }
        }

        var body: some View {
            HStack(alignment: .center, spacing: 48) {
                if let profileURL {
                    QRCodeView(profileURL)
                        .frame(width: qrSize, height: qrSize)
                }
                VStack(alignment: .leading, spacing: Tokens.Spacing.lg) {
                    Text(L10n.profilePicturesOnPhone)
                        .typeRole(Tokens.TypeRamp.section)
                        .foregroundStyle(Tokens.Palette.text)
                        .accessibilityAddTraits(.isHeader)
                    Text(L10n.profilePicturesOnPhoneHint)
                        .typeRole(Tokens.TypeRamp.body)
                        .foregroundStyle(Tokens.Palette.muted)
                        .fixedSize(horizontal: false, vertical: true)
                    if let profileURL {
                        Text(PairingPanel.displayed(profileURL))
                            .typeRole(Tokens.TypeRamp.caption)
                            .foregroundStyle(Tokens.Palette.muted)
                    }
                    HStack(spacing: Tokens.Spacing.lg) {
                        if user?.avatarId != nil {
                            Button(L10n.profileRemovePicture, systemImage: "trash") {
                                core.send(.imageRemoved(ImageSlotRef(slot: .avatar)))
                            }
                        }
                        if user?.bannerId != nil {
                            Button(L10n.profileBannerRemove, systemImage: "trash") {
                                core.send(.imageRemoved(ImageSlotRef(slot: .banner)))
                            }
                        }
                    }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .tvFocusSection()
        }
    }
#endif

extension View {
    /// Holds a surface open while the view is up, when there is one to hold.
    @ViewBuilder fileprivate func opening(_ surface: Surface?) -> some View {
        if let surface {
            coreScreen(surface)
        } else {
            self
        }
    }
}
