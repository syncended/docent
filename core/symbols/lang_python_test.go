package symbols

import "testing"

func TestExtractPython(t *testing.T) {
	src := `
import dataclasses

MAX_ITEMS: int = 100
default_currency = "EUR"
first, second = 1, 2

if DEBUG:
    log_level = "debug"


@dataclasses.dataclass
class Order(Base):
    TABLE = "orders"
    total: int
    status = "new"

    @property
    def amount(self):
        rate = 1
        def convert(x):
            nested = x
            return nested
        return convert(self.total) * rate

    async def submit(self):
        self.submitted = True


def make_order(total):
    order = Order(total)
    class Local:
        pass
    return order


@cached
async def fetch_orders():
    pass


key = lambda o: o.total
`
	checkExtract(t, "python", src, []string{
		"MAX_ITEMS constant",
		"default_currency variable",
		"first variable",
		"second variable",
		"log_level variable",
		"Order class",
		"Order.TABLE constant",
		"Order.total field",
		"Order.status field",
		"Order.amount method",
		"Order.submit method",
		"make_order function",
		"fetch_orders function",
		"key variable",
	}, []string{"rate", "convert", "nested", "submitted", "order", "Local", "o"})
}
