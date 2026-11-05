package symbols

import "testing"

func TestExtractGroovy(t *testing.T) {
	src := `package com.example.shop

import groovy.transform.CompileStatic

@CompileStatic
class Order extends Entity {
    static final int MAX_ITEMS = 10
    private String id
    def total = 0
    List<String> items

    int compareTo(Order other) {
        def delta = 1
        return id <=> other.id
    }

    static Order of(String id) { new Order(id) }

    def process() {
        items.each { item -> println item }
    }
}

class Invoice extends Document {
    String number
}

class Cart implements Iterable<Item> {
    Iterator<Item> iterator() { items.iterator() }
}

interface Shape extends Drawable {
    double area()
}

enum Color {
    RED, GREEN
}

trait Named {
    String name
}

def helper(x) {
    def doubled = x * 2
    return doubled
}

String describe(int a) { a.toString() }

def version = '1.0'

task release {
    doLast { def stamp = 1 }
}
`
	want := []string{
		"Order class",
		"Order.MAX_ITEMS field",
		"Order.id field",
		"Order.total field",
		"Order.items field",
		"Order.compareTo method",
		"Order.of method",
		"Order.process method",
		"Invoice class",
		"Invoice.number field",
		"Cart class",
		"Shape interface",
		"Shape.area method",
		"Color enum",
		"Named interface",
		"helper function",
		"describe function",
		"version variable",
	}
	not := []string{"Entity", "Iterable", "Document", "Drawable", "delta", "item", "doubled", "release", "stamp", "RED"}
	checkExtract(t, "groovy", src, want, not)
}
