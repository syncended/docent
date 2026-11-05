package symbols

import "testing"

func TestExtractCSharp(t *testing.T) {
	checkExtract(t, "csharp", `
using System;

namespace Shop.Core
{
    public class Order<T> : IOrder
    {
        public const int Max = 10;
        private readonly string id, label;
        public int Total { get; set; }
        public event EventHandler Changed;

        public Order() { int ctorLocal = 0; }
        public int Compute(int x)
        {
            var tmp = x;
            Func<int, int> square = y => { int inside = y; return inside * inside; };
            int Helper() => tmp;
            return Helper();
        }

        public enum Status { New, Done = 2 }
    }

    public struct Point { public int X; }
    public interface IOrder { int Compute(int x); string Name { get; } }
    public record Person(string First, string Last);
    public record struct Coord(int X, int Y);
    public delegate void Handler(object sender);
}

namespace Shop.Other;

class Repo { void Save() { } }
`, []string{
		"Shop.Core namespace",
		"Shop.Core.Order class", "Order.Max constant", "Order.id field", "Order.label field",
		"Order.Total field", "Order.Changed field", "Order.Compute method",
		"Order.Status enum", "Status.New enum_value", "Status.Done enum_value",
		"Shop.Core.Point struct", "Point.X field",
		"Shop.Core.IOrder interface", "IOrder.Compute method", "IOrder.Name field",
		"Shop.Core.Person class", "Shop.Core.Coord struct", "Shop.Core.Handler type",
		"Shop.Other namespace", "Shop.Other.Repo class", "Repo.Save method",
	}, []string{"ctorLocal", "tmp", "square", "inside", "Helper"})
}
