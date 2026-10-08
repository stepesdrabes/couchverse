import CoreGraphics
import Foundation
import ImageIO
import Testing
import UniformTypeIdentifiers

@testable import CouchverseCore

struct MultipartFormTests {
    @Test func theFileIsTheFormsOnePart() {
        let file = PickedFile(name: "7F3A.jpg", data: Data([0xFF, 0xD8, 0xFF]))
        let form = MultipartForm(field: "file", file: file, boundary: "b0undary")

        #expect(form.contentType == "multipart/form-data; boundary=b0undary")
        var expected = Data(
            """
            --b0undary\r
            Content-Disposition: form-data; name="file"; filename="7F3A.jpg"\r
            Content-Type: image/jpeg\r
            \r

            """.utf8)
        expected.append(file.data)
        expected.append(Data("\r\n--b0undary--\r\n".utf8))
        #expect(form.body == expected)
    }

    @Test func aNameCannotBreakOutOfItsHeader() {
        let form = MultipartForm(
            field: "file", file: PickedFile(name: "a\"b\r\nc.png", data: Data()), boundary: "x")
        let text = String(decoding: form.body, as: UTF8.self)
        #expect(text.contains(#"filename="abc.png""#))
        #expect(text.contains("Content-Type: image/png"))
    }
}

struct UploadFilesTests {
    let files = UploadFiles(directory: FileManager.default.temporaryDirectory.appending(path: UUID().uuidString))

    @Test func aPickedPhotoBecomesAJPEGNoLargerThanTheSlotNeeds() async throws {
        defer { try? FileManager.default.removeItem(at: files.directory) }
        let handle = try await files.holdImage(try Self.png(width: 3000, height: 1000), maxPixels: 1024)

        let file = try #require(UploadFiles.open(handle))
        #expect(file.name.hasSuffix(".jpg"))
        #expect(file.contentType == "image/jpeg")
        #expect(try Self.size(of: file.data) == CGSize(width: 1024, height: 341))
    }

    @Test func aSmallPictureKeepsItsSize() async throws {
        defer { try? FileManager.default.removeItem(at: files.directory) }
        let handle = try await files.holdImage(try Self.png(width: 40, height: 20), maxPixels: 1024)
        let file = try #require(UploadFiles.open(handle))
        #expect(try Self.size(of: file.data) == CGSize(width: 40, height: 20))
    }

    @Test func anotherPickReplacesTheFirst() async throws {
        defer { try? FileManager.default.removeItem(at: files.directory) }
        let first = try await files.holdImage(try Self.png(width: 10, height: 10), maxPixels: 64)
        let second = try await files.holdImage(try Self.png(width: 12, height: 12), maxPixels: 64)
        #expect(UploadFiles.open(first) == nil)
        #expect(UploadFiles.open(second) != nil)
    }

    @Test func whatIsNotAnImageIsRefused() async {
        await #expect(throws: UploadFileError.self) {
            try await files.holdImage(Data("not a picture".utf8), maxPixels: 64)
        }
    }

    @Test(arguments: ["", "pick-1", "https://media.example.com/a.jpg", "file:///nowhere/at/all.jpg"])
    func onlyAFileThatIsThereCanBeOpened(handle: String) {
        #expect(UploadFiles.open(handle) == nil)
    }

    static func png(width: Int, height: Int) throws -> Data {
        let context = try #require(
            CGContext(
                data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
                space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.setFillColor(CGColor(red: 0.2, green: 0.4, blue: 0.6, alpha: 1))
        context.fill(CGRect(x: 0, y: 0, width: width, height: height))
        let image = try #require(context.makeImage())
        let output = NSMutableData()
        let destination = try #require(
            CGImageDestinationCreateWithData(output, UTType.png.identifier as CFString, 1, nil))
        CGImageDestinationAddImage(destination, image, nil)
        #expect(CGImageDestinationFinalize(destination))
        return output as Data
    }

    static func size(of data: Data) throws -> CGSize {
        let source = try #require(CGImageSourceCreateWithData(data as CFData, nil))
        let properties = try #require(CGImageSourceCopyPropertiesAtIndex(source, 0, nil) as? [CFString: Any])
        let width = try #require(properties[kCGImagePropertyPixelWidth] as? Int)
        let height = try #require(properties[kCGImagePropertyPixelHeight] as? Int)
        return CGSize(width: width, height: height)
    }
}

/// Uploads share `HTTPExecutorTests`' stubbed session, so they run in its serialized suite.
extension HTTPExecutorTests {
    @Test func anUploadSendsThePickedFileAsAForm() async throws {
        let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let handle = try await UploadFiles(directory: directory).holdImage(
            try UploadFilesTests.png(width: 8, height: 8), maxPixels: 64)
        let url = "https://tv.home/api/v1/me/avatar"
        StubProtocol.lock.withLock {
            StubProtocol.answers[url] = .response(status: 200, body: #"{"username":"nora"}"#)
        }
        let request = HttpRequest(
            method: "POST", url: url,
            headers: [
                HttpHeader(name: "Accept", value: "application/json"),
                HttpHeader(name: "Authorization", value: "Bearer t"),
            ])

        let output = await executor.upload(UploadRequest(request: request, file: handle, field: "file"))

        #expect(output == .http(HttpResponse(status: 200, body: #"{"username":"nora"}"#)))
        let sent = try #require(StubProtocol.lock.withLock { StubProtocol.seen.last })
        #expect(sent.url?.absoluteString == url)
        #expect(sent.httpMethod == "POST")
        #expect(sent.value(forHTTPHeaderField: "Authorization") == "Bearer t")
        let contentType = try #require(sent.value(forHTTPHeaderField: "Content-Type"))
        #expect(contentType.hasPrefix("multipart/form-data; boundary="))
        let body = try #require(sent.httpBody)
        let file = try #require(UploadFiles.open(handle))
        let name = URL(string: handle)?.lastPathComponent ?? ""
        #expect(String(decoding: body, as: UTF8.self).contains(#"name="file"; filename="\#(name)""#))
        #expect(body.range(of: file.data) != nil)
    }

    @Test func anUploadOfAFileThatIsGoneFailsWithoutARequest() async {
        let before = StubProtocol.lock.withLock { StubProtocol.seen.count }
        let request = HttpRequest(method: "POST", url: "https://tv.home/api/v1/me/banner", headers: [])
        let output = await executor.upload(
            UploadRequest(request: request, file: "file:///nowhere/gone.jpg", field: "file"))
        guard case .httpFailed(let failure) = output else {
            Issue.record("expected a failure, got \(output)")
            return
        }
        #expect(failure.kind == .other)
        #expect(StubProtocol.lock.withLock { StubProtocol.seen.count } == before)
    }
}
