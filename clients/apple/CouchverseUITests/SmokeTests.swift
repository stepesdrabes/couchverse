import XCTest

/// A fresh install on an iPhone or iPad simulator, without a server: the app starts at the
/// welcome screen and the core's address checks reach the add-server form.
final class SmokeTests: XCTestCase {
    @MainActor
    func testAFreshInstallWelcomesAndChecksServerAddresses() {
        let app = launchFresh()
        XCTAssert(app.staticTexts["Welcome to Couchverse"].waitForExistence(timeout: 15))
        app.buttons["Add a server"].tap()

        let field = app.textFields["Server address"]
        XCTAssert(field.waitForExistence(timeout: 5))
        field.tap()
        field.typeText("my server\n")
        XCTAssert(
            app.staticTexts["Enter an address like media.example.com or 192.168.1.5:8080."]
                .waitForExistence(timeout: 5))

        // nothing listens on the discard port, so both https and http fail fast
        field.tap()
        field.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: 12) + "127.0.0.1:9\n")
        XCTAssert(
            app.staticTexts["Can't reach the server. Check the address and your network."]
                .waitForExistence(timeout: 20))
    }
}

extension XCTestCase {
    /// Launches the app with in-memory stores (a fresh install every time) in English.
    @MainActor
    func launchFresh() -> XCUIApplication {
        continueAfterFailure = false
        let app = XCUIApplication()
        app.launchArguments = ["-uiTesting", "-AppleLanguages", "(en)", "-AppleLocale", "en_US"]
        app.launch()
        return app
    }
}
