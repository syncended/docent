package symbols

import "testing"

func TestExtractPHP(t *testing.T) {
	src := `<?php
namespace App\Billing;

const VERSION = "1.0";

interface Payable {
    const CURRENCY = "EUR";
    public function total(): float;
}

trait Loggable {
    protected $logger;
    public function log(string $message): void {
        $prefix = "[log] ";
        $format = function ($m) { return $m; };
    }
}

enum Status: string {
    case Open = 'open';
    case Paid = 'paid';
}

final class Invoice implements Payable {
    use Loggable;
    public const MAX_LINES = 100, MIN_LINES = 1;
    private int $number;
    public static ?string $series = null, $prefix;

    public function __construct(private float $amount) {}

    public function total(): float {
        $tax = 0.2;
        $printer = new class { public function render() {} };
        return $this->amount * (1 + $tax);
    }
}

if (!function_exists('format_money')) {
    function format_money(float $value): string {
        function nested() {}
        return number_format($value, 2);
    }
}
`
	want := []string{
		"App\\Billing namespace",
		"VERSION constant",
		"Payable interface",
		"Payable.CURRENCY constant",
		"Payable.total method",
		"Loggable interface",
		"Loggable.logger field",
		"Loggable.log method",
		"Status enum",
		"Status.Open enum_value",
		"Status.Paid enum_value",
		"Invoice class",
		"Invoice.MAX_LINES constant",
		"Invoice.MIN_LINES constant",
		"Invoice.number field",
		"Invoice.series field",
		"Invoice.prefix field",
		"Invoice.__construct method",
		"Invoice.amount field",
		"Invoice.total method",
		"format_money function",
	}
	not := []string{"message", "format", "tax", "printer", "render", "nested", "value"}
	checkExtract(t, "php", src, want, not)

	braced := `<?php
namespace App\Util {
    function slug($s) { return $s; }
    class Str {}
}
`
	checkExtract(t, "php", braced, []string{"App\\Util namespace", "App\\Util.slug function", "App\\Util.Str class"}, nil)
}
