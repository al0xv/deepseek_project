import XCTest
@testable import DSRemote

final class PairingCodeTests: XCTestCase {
    func testValidSixDigits() {
        XCTAssertEqual(PairingCode.normalize("472913"), "472913")
        XCTAssertEqual(PairingCode.normalize("472 913"), "472913")
        XCTAssertEqual(PairingCode.normalize("472-913"), "472913")
    }

    func testTooShort() {
        XCTAssertNil(PairingCode.normalize("123"))
    }

    func testTooLong() {
        XCTAssertNil(PairingCode.normalize("1234567"))
    }

    func testNonDigits() {
        XCTAssertNil(PairingCode.normalize("abcdef"))
    }

    func testEmpty() {
        XCTAssertNil(PairingCode.normalize(""))
    }
}
