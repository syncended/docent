package symbols

import "testing"

func TestExtractRuby(t *testing.T) {
	src := `
VERSION = "1.0"

module Shop
  DEFAULT_CURRENCY = "EUR"

  def self.configure
    config = {}
    config
  end

  def helper
    total = 0
    total
  end

  class Order < Base
    STATUSES = %w[new paid].freeze
    MIN, MAX = 1, 10

    def initialize(total)
      @total = total
    end

    def total=(value)
      @total = value
    end

    def self.build(attrs)
      order = new(attrs)
      order
    end

    class << self
      def find(id)
        id
      end
    end
  end
end

class Shop::Invoice
end

Shop::LIMIT = 5

def run_script
  result = 1
  result
end
`
	checkExtract(t, "ruby", src, []string{
		"VERSION constant",
		"Shop namespace",
		"Shop.DEFAULT_CURRENCY constant",
		"Shop.configure method",
		"Shop.helper method",
		"Shop.Order class",
		"Order.STATUSES constant",
		"Order.MIN constant",
		"Order.MAX constant",
		"Order.initialize method",
		"Order.total= method",
		"Order.build method",
		"Order.find method",
		"Invoice class",
		"LIMIT constant",
		"run_script function",
	}, []string{"config", "total", "order", "result", "id"})
}
