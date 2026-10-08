import AppIntents
import CouchverseFeatures

// Each intent opens the app and leaves an `OpenRequest`, which the app acts on once an account is
// signed in, the same way as a link.

struct OpenTitleIntent: AppIntent {
    static let title = LocalizedStringResource("intent_open_title")
    static let description = IntentDescription(LocalizedStringResource("intent_open_title_description"))
    static let supportedModes: IntentModes = .foreground

    @Parameter(
        title: LocalizedStringResource("intent_title"),
        requestValueDialog: IntentDialog(LocalizedStringResource("intent_title_prompt")))
    var title: TitleEntity

    @Dependency private var requests: OpenRequests

    @MainActor
    func perform() async throws -> some IntentResult {
        requests.open(.title(slug: title.id))
        return .result()
    }
}

struct ContinueWatchingIntent: AppIntent {
    static let title = LocalizedStringResource("intent_continue_watching")
    static let description = IntentDescription(LocalizedStringResource("intent_continue_watching_description"))
    static let supportedModes: IntentModes = .foreground

    /// Without one, the first title in Continue Watching.
    @Parameter(title: LocalizedStringResource("intent_in_progress"))
    var item: ContinueEntity?

    @Dependency private var requests: OpenRequests

    @MainActor
    func perform() async throws -> some IntentResult {
        requests.open(item.flatMap { OpenRequest(link: $0.id) } ?? .continueWatching)
        return .result()
    }
}

struct OpenMyListIntent: AppIntent {
    static let title = LocalizedStringResource("intent_open_my_list")
    static let description = IntentDescription(LocalizedStringResource("intent_open_my_list_description"))
    static let supportedModes: IntentModes = .foreground

    @Dependency private var requests: OpenRequests

    @MainActor
    func perform() async throws -> some IntentResult {
        requests.open(.myList)
        return .result()
    }
}

struct JoinCouchIntent: AppIntent {
    static let title = LocalizedStringResource("intent_join_couch")
    static let description = IntentDescription(LocalizedStringResource("intent_join_couch_description"))
    static let supportedModes: IntentModes = .foreground

    /// Without one, the join screen opens empty, ready for the code or a scan.
    @Parameter(
        title: LocalizedStringResource("intent_couch_code"),
        inputOptions: String.IntentInputOptions(keyboardType: .numberPad))
    var code: String?

    @Dependency private var requests: OpenRequests

    @MainActor
    func perform() async throws -> some IntentResult {
        requests.open(.joinCouch(code))
        return .result()
    }
}
