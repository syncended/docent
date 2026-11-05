package symbols

import "testing"

func TestExtractScala(t *testing.T) {
	checkExtract(t, "scala", `
package shop.core

package object util { val Version = "1" }

class Order(val id: String, var total: Int, note: String) extends Base {
  val fee: Int = 1
  var count, limit = 0
  def compute(x: Int): Int = { val local = x; def inner() = 1; local + inner() }
  type Id = String
}

case class Point(x: Int, y: Int)
object Order { def apply(): Order = ???; val Default = 1 }
case object Empty
trait Repo { def get(id: String): Order; val name: String }
abstract class Base

enum Color:
  case Red, Green
  case Mix(r: Int)

class Report:
  def render: String =
    val header = "x"
    header

package nested {
  class Inner
}

type Alias = Int
val topLevel = 1
given defaultOrdering: Ordering[Int] = Ordering.Int
extension (s: String) def shout: String = s.toUpperCase
def describe(x: Int) = x match
  case 1 =>
    val inCase = "one"
    inCase
  case _ => "other"
def run(): Unit = List(1).foreach { n => val inLambda = n }
`, []string{
		"util object", "util.Version field",
		"Order class", "Order.id field", "Order.total field", "Order.fee field",
		"Order.count field", "Order.limit field", "Order.compute method", "Order.Id type",
		"Point class", "Order object", "Order.apply method", "Order.Default field",
		"Empty object", "Repo interface", "Repo.get method", "Repo.name field", "Base class",
		"Color enum", "Color.Red enum_value", "Color.Green enum_value", "Color.Mix enum_value",
		"Report class", "Report.render method",
		"nested namespace", "nested.Inner class",
		"Alias type", "topLevel variable", "defaultOrdering variable",
		"shout function", "describe function", "run function",
	}, []string{"local", "inner", "note", "header", "inCase", "inLambda", "r"})
}
