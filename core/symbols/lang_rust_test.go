package symbols

import "testing"

func TestExtractRust(t *testing.T) {
	checkExtract(t, "rust", `
pub mod store {
    pub struct Order<T> { pub id: u64, total: T }
    pub struct Marker;
    pub enum Status { New, Done(u8), Failed { code: i32 } }
    union Bits { int: u32, float: f32 }

    pub trait Repo {
        type Item;
        const LIMIT: usize;
        fn get(&self, id: u64) -> Option<Self::Item>;
        fn name(&self) -> &str { let fallback = "repo"; fallback }
    }

    impl<T> Order<T> {
        pub fn total(&self) -> &T { let local = 1; fn helper() {} &self.total }
    }
    impl Repo for Order<u8> {
        type Item = u8;
        const LIMIT: usize = 8;
        fn get(&self, id: u64) -> Option<u8> { None }
        fn name(&self) -> &str { "order" }
    }
    impl std::fmt::Display for crate::store::Marker {
        fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result { Ok(()) }
    }
    impl<'a> From<&'a str> for &'a Marker {
        fn from(s: &'a str) -> Self { &Marker }
    }

    pub const MAX: usize = 10;
    static mut COUNT: u32 = 0;
    pub type Id = u64;
}

macro_rules! square { ($x:expr) => { $x * $x }; }

pub fn run() { let callback = |x: i32| { let inside = x; inside }; }
`, []string{
		"store namespace",
		"store.Order struct", "Order.id field", "Order.total field",
		"store.Marker struct",
		"store.Status enum", "Status.New enum_value", "Status.Done enum_value", "Status.Failed enum_value",
		"store.Bits union", "Bits.int field", "Bits.float field",
		"store.Repo interface", "Repo.Item type", "Repo.LIMIT constant", "Repo.get method", "Repo.name method",
		"Order.total method", "Order.Item type", "Order.LIMIT constant", "Order.get method", "Order.name method",
		"Marker.fmt method", "Marker.from method",
		"store.MAX constant", "store.COUNT constant", "store.Id type",
		"square function", "run function",
	}, []string{"local", "helper", "fallback", "callback", "inside", "code", "Display", "From"})
}
