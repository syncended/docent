package symbols

import "testing"

func TestExtractSwift(t *testing.T) {
	checkExtract(t, "swift", `
import Foundation

class Order: Codable {
    var id: String = ""
    let total: Int
    static var shared = Order(total: 0)
    var doubled: Int { let factor = 2; return total * factor }
    var note = "" { didSet { let changed = true } }

    init(total: Int) { self.total = total; let initLocal = 1 }
    func compute() -> Int { var tmp = 0; return tmp }

    struct Line { var x, y: Double }
}

enum Status { case new, done; case failed(code: Int) }

protocol Repo {
    func get(id: String) -> Order?
    var name: String { get }
}

actor Counter { var value = 0 }

extension Order: Equatable {
    func describe() -> String { "" }
}
extension Swift.Array {
    func second() -> Element? { nil }
}

typealias OrderID = String
let defaultTotal = 1
func run() { let handler = { let inside = 1 } }
`, []string{
		"Order class", "Order.id field", "Order.total field", "Order.shared field",
		"Order.doubled field", "Order.note field", "Order.compute method",
		"Order.Line struct", "Line.x field", "Line.y field",
		"Status enum", "Status.new enum_value", "Status.done enum_value", "Status.failed enum_value",
		"Repo interface", "Repo.get method", "Repo.name field",
		"Counter class", "Counter.value field",
		"Order.describe method", "Array.second method",
		"OrderID type", "defaultTotal variable", "run function",
	}, []string{"factor", "changed", "initLocal", "tmp", "handler", "inside", "init", "Equatable", "Swift"})
}
