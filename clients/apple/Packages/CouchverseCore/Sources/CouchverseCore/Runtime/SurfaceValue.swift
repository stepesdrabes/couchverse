import Foundation

/// A decoded view model, tagged with the surface it belongs to.
public enum SurfaceValue: Sendable, Hashable {
    case app(AppView)
    case servers(ServersView)
    case accounts(AccountsView)
    case signIn(SignInView)
    case devices(DevicesView)
    case pairingApproval(PairingApprovalView)
    case session(SessionView)

    /// Every surface the runtime publishes; `Markdown` is read on demand instead, and the
    /// catalog's surfaces get their screens in a later slice.
    static let published: [Surface] = [
        .app, .servers, .accounts, .signIn, .devices, .pairingApproval, .session,
    ]

    var surface: Surface {
        switch self {
        case .app: .app
        case .servers: .servers
        case .accounts: .accounts
        case .signIn: .signIn
        case .devices: .devices
        case .pairingApproval: .pairingApproval
        case .session: .session
        }
    }

    /// Decodes a surface's JSON. S2 measured `JSONDecoder`, not the core, as the dominant cost of a
    /// render, so batches are decoded off the main actor.
    static func decode(_ surface: Surface, _ json: String) throws -> SurfaceValue? {
        let decoder = JSONDecoder()
        let data = Data(json.utf8)
        return switch surface {
        case .app: .app(try decoder.decode(AppView.self, from: data))
        case .servers: .servers(try decoder.decode(ServersView.self, from: data))
        case .accounts: .accounts(try decoder.decode(AccountsView.self, from: data))
        case .signIn: .signIn(try decoder.decode(SignInView.self, from: data))
        case .devices: .devices(try decoder.decode(DevicesView.self, from: data))
        case .pairingApproval: .pairingApproval(try decoder.decode(PairingApprovalView.self, from: data))
        case .session: .session(try decoder.decode(SessionView.self, from: data))
        default: nil
        }
    }

    @concurrent
    static func decodeBatch(_ payloads: [RenderedSurface]) async -> [SurfaceValue] {
        payloads.compactMap { payload in
            do {
                return try decode(payload.surface, payload.json)
            } catch {
                coreLog.fault("undecodable \(String(describing: payload.surface)) view: \(error)")
                return nil
            }
        }
    }
}

/// A surface's view model as the core rendered it, not yet decoded.
struct RenderedSurface: Sendable {
    let surface: Surface
    let json: String
}
