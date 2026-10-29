import XCTest
@testable import Impressipies

final class AuthLinkTests: XCTestCase {
    func testParsesTheEmailedLink() throws {
        let link = try XCTUnwrap(AuthLink(URL(string: "https://impressipes.bancroft.io/auth/verify?t=abc-DEF_123")!))
        XCTAssertEqual(link.token, "abc-DEF_123")
        XCTAssertEqual(link.server, URL(string: "https://impressipes.bancroft.io")!)
    }

    func testKeepsThePortOfALocalServer() throws {
        let link = try XCTUnwrap(AuthLink(URL(string: "http://localhost:8080/auth/verify?t=x")!))
        XCTAssertEqual(link.server, URL(string: "http://localhost:8080")!)
    }

    func testIgnoresOtherURLs() {
        XCTAssertNil(AuthLink(URL(string: "https://impressipes.bancroft.io/recipes?t=x")!))
        XCTAssertNil(AuthLink(URL(string: "https://impressipes.bancroft.io/auth/verify")!))
        XCTAssertNil(AuthLink(URL(string: "https://impressipes.bancroft.io/auth/verify?t=")!))
        XCTAssertNil(AuthLink(URL(string: "impressipes://auth/verify?t=x")!))
    }
}
