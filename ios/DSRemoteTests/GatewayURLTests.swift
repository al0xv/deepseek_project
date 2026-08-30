import XCTest
@testable import DSRemote

final class GatewayURLTests: XCTestCase {
    func testValidURLs() {
        XCTAssertNotNil(GatewayURLValidator.parse("http://192.168.1.42:8080"))
        XCTAssertNotNil(GatewayURLValidator.parse("http://localhost:8080"))
        XCTAssertNotNil(GatewayURLValidator.parse("https://example.com"))
    }

    func testInvalidURLs() {
        XCTAssertNil(GatewayURLValidator.parse("abc"))
        XCTAssertNil(GatewayURLValidator.parse(""))
        XCTAssertNil(GatewayURLValidator.parse("http://"))
        XCTAssertNil(GatewayURLValidator.parse("ftp://example.com"))
        XCTAssertNil(GatewayURLValidator.parse("   "))
    }
}
