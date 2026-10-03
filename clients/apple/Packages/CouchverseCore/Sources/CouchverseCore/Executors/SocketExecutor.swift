import Foundation

/// The core's WebSockets over URLSession. An open resolves `socketOpened` once the server answers
/// a ping, then `socketText` per frame, and ends with exactly one `socketClosed`; a close the core
/// asks for is not reported back (it has already forgotten the socket).
@MainActor
public final class SocketExecutor: SocketExecuting {
    private let session: URLSession
    private var sockets: [UInt64: URLSessionWebSocketTask] = [:]

    public init(session: URLSession = URLSession(configuration: .ephemeral)) {
        self.session = session
    }

    public func open(id: UInt64, request: SocketOpen, deliver: @escaping @MainActor (EffectOutput) -> Void) {
        guard let url = URL(string: request.url) else {
            deliver(.socketClosed(SocketClosed(code: Self.abnormal, reason: "not a URL: \(request.url)")))
            return
        }
        var urlRequest = URLRequest(url: url)
        for header in request.headers {
            urlRequest.setValue(header.value, forHTTPHeaderField: header.name)
        }
        let task = session.webSocketTask(with: urlRequest)
        sockets[id] = task
        task.resume()
        Task { [weak self] in
            // URLSession reports the handshake only to a delegate; a pong proves the socket is open
            let opened = await withCheckedContinuation { continuation in
                task.sendPing { continuation.resume(returning: $0 == nil) }
            }
            if opened {
                deliver(.socketOpened)
                while let text = await Self.receive(task) {
                    deliver(.socketText(SocketText(text: text)))
                }
            }
            guard self?.sockets.removeValue(forKey: id) != nil else { return }
            let code = task.closeCode == .invalid ? Self.abnormal : UInt16(clamping: task.closeCode.rawValue)
            let reason = task.closeReason.map { String(decoding: $0, as: UTF8.self) }
            deliver(.socketClosed(SocketClosed(code: code, reason: reason)))
        }
    }

    public func send(socket: UInt64, text: String) {
        guard let task = sockets[socket] else { return }
        // a failed write surfaces as the receive loop ending, which closes the socket
        task.send(.string(text)) { _ in }
    }

    public func close(socket: UInt64) {
        sockets.removeValue(forKey: socket)?.cancel(with: .normalClosure, reason: nil)
    }

    /// The next text frame, or `nil` once the socket is closed. Binary frames are not part of the
    /// couch protocol and are skipped.
    private static func receive(_ task: URLSessionWebSocketTask) async -> String? {
        while true {
            switch try? await task.receive() {
            case .string(let text): return text
            case .data: continue
            case nil: return nil
            @unknown default: continue
            }
        }
    }

    /// RFC 6455's "closed without a close frame".
    static let abnormal: UInt16 = 1006
}
