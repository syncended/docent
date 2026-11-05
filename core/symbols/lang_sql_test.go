package symbols

import "testing"

func TestExtractSQL(t *testing.T) {
	src := `
CREATE SCHEMA billing;

CREATE TABLE billing.invoices (
  id bigserial PRIMARY KEY,
  customer_id bigint NOT NULL REFERENCES customers(id),
  total numeric(12, 2) NOT NULL DEFAULT 0,
  CONSTRAINT positive_total CHECK (total >= 0)
);

CREATE VIEW open_invoices AS SELECT * FROM billing.invoices WHERE total > 0;
CREATE MATERIALIZED VIEW revenue AS SELECT sum(total) FROM billing.invoices;

CREATE OR REPLACE FUNCTION invoice_total(inv bigint) RETURNS numeric LANGUAGE plpgsql AS $$
BEGIN
  CREATE TEMPORARY TABLE scratch (x int);
  RETURN 0;
END;
$$;

CREATE TYPE invoice_status AS ENUM ('draft', 'sent', 'paid');
CREATE TYPE money_pair AS (amount numeric, currency text);

CREATE INDEX invoices_customer_idx ON billing.invoices (customer_id);
CREATE SEQUENCE invoice_seq START 100;

ALTER TABLE customers ADD COLUMN nickname text;
`
	checkExtract(t, "sql", src, []string{
		"billing namespace",
		"invoices table",
		"invoices.id field",
		"invoices.customer_id field",
		"invoices.total field",
		"invoices.positive_total constant",
		"open_invoices table",
		"revenue table",
		"invoice_total function",
		"invoice_status enum",
		"money_pair struct",
		"money_pair.amount field",
		"money_pair.currency field",
		"invoices_customer_idx constant",
		"invoice_seq constant",
	}, []string{"scratch", "x", "inv", "customers", "nickname", "sum"})
}
