import Foundation

/// URLSession for the core's API calls: system TLS trust and proxies, no cookies (devices
/// authenticate with bearer tokens) and no cache (the core decides what is fresh).
public struct HTTPExecutor: HTTPExecuting {
    private let session: URLSession

    public init(session: URLSession) {
        self.session = session
    }

    public init(userAgent: String, timeout: TimeInterval = 15) {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.timeoutIntervalForRequest = timeout
        configuration.waitsForConnectivity = false
        configuration.httpShouldSetCookies = false
        configuration.httpCookieAcceptPolicy = .never
        configuration.requestCachePolicy = .reloadIgnoringLocalCacheData
        configuration.urlCache = nil
        configuration.httpAdditionalHeaders = ["User-Agent": userAgent]
        self.init(session: URLSession(configuration: configuration))
    }

    @concurrent
    public func perform(_ request: HttpRequest) async -> EffectOutput {
        guard var urlRequest = Self.urlRequest(request) else {
            return .httpFailed(HttpFailure(kind: .other, message: "not a URL: \(request.url)"))
        }
        urlRequest.httpBody = request.body.map { Data($0.utf8) }
        return await send(urlRequest)
    }

    @concurrent
    public func upload(_ upload: UploadRequest) async -> EffectOutput {
        guard var urlRequest = Self.urlRequest(upload.request) else {
            return .httpFailed(HttpFailure(kind: .other, message: "not a URL: \(upload.request.url)"))
        }
        guard let file = UploadFiles.open(upload.file) else {
            return .httpFailed(HttpFailure(kind: .other, message: "the picked file can no longer be read"))
        }
        let form = MultipartForm(field: upload.field, file: file)
        urlRequest.setValue(form.contentType, forHTTPHeaderField: "Content-Type")
        urlRequest.httpBody = form.body
        return await send(urlRequest)
    }

    private static func urlRequest(_ request: HttpRequest) -> URLRequest? {
        guard let url = URL(string: request.url) else { return nil }
        var urlRequest = URLRequest(url: url)
        urlRequest.httpMethod = request.method
        for header in request.headers {
            urlRequest.setValue(header.value, forHTTPHeaderField: header.name)
        }
        return urlRequest
    }

    private func send(_ urlRequest: URLRequest) async -> EffectOutput {
        do {
            let (data, response) = try await session.data(for: urlRequest)
            let status = (response as? HTTPURLResponse)?.statusCode ?? 0
            return .http(
                HttpResponse(status: UInt16(clamping: status), body: String(decoding: data, as: UTF8.self)))
        } catch let error as URLError {
            return .httpFailed(HttpFailure(kind: Self.kind(of: error), message: error.localizedDescription))
        } catch {
            return .httpFailed(HttpFailure(kind: .other, message: error.localizedDescription))
        }
    }

    /// The core retries the next candidate address on any of these, and shows the kind.
    static func kind(of error: URLError) -> HttpFailureKind {
        switch error.code {
        case .notConnectedToInternet, .networkConnectionLost, .cannotFindHost, .cannotConnectToHost,
            .dnsLookupFailed, .dataNotAllowed, .internationalRoamingOff, .callIsActive:
            .offline
        case .timedOut:
            .timeout
        case .secureConnectionFailed, .serverCertificateHasBadDate, .serverCertificateUntrusted,
            .serverCertificateHasUnknownRoot, .serverCertificateNotYetValid, .clientCertificateRejected,
            .clientCertificateRequired, .appTransportSecurityRequiresSecureConnection:
            .tls
        default:
            .other
        }
    }
}
