package symbols

import "testing"

func TestExtractOCaml(t *testing.T) {
	src := `open Printf

type color = Red | Green of int | Blue

type point = { x : float; mutable y : float }

type 'a tree = Leaf | Node of 'a tree * 'a * 'a tree
and forest = int tree list

type id = int

exception Parse_error of string

external length : string -> int = "caml_string_length"

let max_size = 100

let add x y = x + y

let incr = fun x -> x + 1

let rec fact n =
  let prev = n - 1 in
  if n = 0 then 1 else n * fact prev

let () = print_endline "ready"

module type Shape = sig
  type t
  val area : t -> float
  val name : string
end

module Circle : Shape = struct
  type t = float
  let area r = 3.14 *. r *. r
  let name = "circle"
end

module Make (S : Shape) = struct
  let run shape = S.area shape
end

class counter = object
  val mutable count = 0
  method incr = let step = 1 in count <- count + step
end
`
	want := []string{
		"color enum",
		"color.Red enum_value",
		"color.Green enum_value",
		"point struct",
		"point.x field",
		"point.y field",
		"tree enum",
		"tree.Leaf enum_value",
		"forest type",
		"id type",
		"Parse_error class",
		"length function",
		"max_size variable",
		"add function",
		"incr function",
		"fact function",
		"Shape interface",
		"Shape.t type",
		"Shape.area method",
		"Shape.name variable",
		"Circle namespace",
		"Circle.t type",
		"Circle.area function",
		"Circle.name variable",
		"Make namespace",
		"Make.run function",
		"counter class",
		"counter.count field",
		"counter.incr method",
	}
	not := []string{"prev", "step", "Printf", "S", "shape", "r"}
	checkExtract(t, "ocaml", src, want, not)
}
