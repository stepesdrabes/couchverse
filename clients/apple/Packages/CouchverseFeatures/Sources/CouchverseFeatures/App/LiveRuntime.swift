import CouchverseCore
import CouchverseDesign
import Foundation
import UIKit

/// Wires the core to this device's executors; the apps call it once at launch.
public enum LiveRuntime {
    /// UI tests launch with this argument to start from a clean install every time.
    public static let uiTestingArgument = "-uiTesting"

    public static func make(arguments: [String] = ProcessInfo.processInfo.arguments) -> CoreRuntime {
        let bundle = Bundle.main.bundleIdentifier ?? "io.stepes.couchverse"
        let ephemeral = arguments.contains(uiTestingArgument)
        let executors = Executors(
            http: HTTPExecutor(userAgent: userAgent),
            timers: TimerExecutor(),
            sockets: SocketExecutor(),
            secureStore: ephemeral ? MemoryStore() : KeychainStore(service: "\(bundle).tokens"),
            store: ephemeral ? MemoryStore() : persistentStore(bundle))
        let config = CoreConfig(
            platform: platform, authMode: .bearer, deviceName: UIDevice.current.name,
            locale: Locale.preferredLanguages.first ?? "en", origin: "")
        do {
            let runtime = try CoreRuntime(config: config, executors: executors)
            L10n.language = runtime.session.language
            runtime.start()
            return runtime
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

    private static var userAgent: String {
        let version = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "0"
        return "Couchverse/\(version) (\(UIDevice.current.systemName) \(UIDevice.current.systemVersion))"
    }
}
