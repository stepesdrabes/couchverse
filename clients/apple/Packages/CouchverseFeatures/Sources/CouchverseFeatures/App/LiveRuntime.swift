import AVFoundation
import CouchverseCore
import CouchverseDesign
import Foundation
import UIKit

/// The running app's services: the core's runtime and the player it drives. The apps keep one
/// and put both into the environment of `CouchverseRoot`.
public struct LiveApp {
    public let runtime: CoreRuntime
    public let player: PlayerController
    /// The background session downloads run in (iPhone and iPad), whose events the app takes
    /// when the system wakes it for them.
    public let downloads: BackgroundTransfers?
    /// What links and intents ask the app to open, for `CouchverseRoot`'s environment.
    public let requests = OpenRequests()
}

/// Wires the core to this device's executors; the apps call it once at launch.
public enum LiveRuntime {
    /// UI tests launch with this argument to start from a clean install every time.
    public static let uiTestingArgument = "-uiTesting"

    public static func make(arguments: [String] = ProcessInfo.processInfo.arguments) -> LiveApp {
        let bundle = Bundle.main.bundleIdentifier ?? "io.stepes.couchverse"
        let ephemeral = arguments.contains(uiTestingArgument)
        let player = PlayerController()
        var executors = Executors(
            http: HTTPExecutor(userAgent: userAgent),
            timers: TimerExecutor(),
            sockets: SocketExecutor(),
            player: player,
            secureStore: ephemeral ? MemoryStore() : KeychainStore(service: "\(bundle).tokens"),
            store: ephemeral ? MemoryStore() : persistentStore(bundle))
        let downloads = downloadTransfers()
        if let downloads {
            executors.downloads = DownloadExecutor(transfers: downloads, files: downloads.files)
        }
        let config = CoreConfig(
            platform: platform, authMode: .bearer, deviceName: UIDevice.current.name,
            locale: Locale.preferredLanguages.first ?? "en", origin: "")
        do {
            // video plays with the ring/silent switch off and keeps its sound in PiP
            try? AVAudioSession.sharedInstance().setCategory(.playback, mode: .moviePlayback)
            let runtime = try CoreRuntime(config: config, executors: executors)
            L10n.language = runtime.session.language
            runtime.start()
            return LiveApp(runtime: runtime, player: player, downloads: downloads)
        } catch {
            // the configuration is built right here; the core rejecting it is a programming error
            fatalError("the core rejected its configuration: \(error)")
        }
    }

    static var platform: Platform {
        #if os(tvOS)
            .tvos
        #else
            UIDevice.current.userInterfaceIdiom == .pad ? .ipados : .ios
        #endif
    }

    private static func persistentStore(_ bundle: String) -> any KeyValueStore {
        #if os(tvOS)
            DefaultsStore(suiteName: "\(bundle).core")
        #else
            FileStore.applicationSupport()
        #endif
    }

    /// The background session the iPhone and iPad app's downloads run in.
    public static var downloadsSession: String {
        "\(Bundle.main.bundleIdentifier ?? "io.stepes.couchverse").downloads"
    }

    /// None on tvOS, whose storage the system may reclaim at any time.
    private static func downloadTransfers() -> BackgroundTransfers? {
        #if os(tvOS)
            nil
        #else
            BackgroundTransfers(
                identifier: downloadsSession, files: DownloadFiles(directory: PlayerController.downloadsDirectory),
                userAgent: userAgent)
        #endif
    }

    private static var userAgent: String {
        let version = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "0"
        return "Couchverse/\(version) (\(UIDevice.current.systemName) \(UIDevice.current.systemVersion))"
    }
}
