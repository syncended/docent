package symbols

import "testing"

func TestExtractCSS(t *testing.T) {
	src := `
:root {
  --accent: #3366ff;
  --gap: calc(var(--unit) * 2);
}

.card {
  color: var(--accent);
  --card-pad: 4px;
}

@media (prefers-color-scheme: dark) {
  :root { --accent-dark: #99bbff; }
}

@keyframes fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

@property --angle {
  syntax: '<angle>';
  inherits: false;
  initial-value: 0deg;
}

@layer base {
  .x { margin: 0; }
}
`
	checkExtract(t, "css", src, []string{
		"--accent variable",
		"--gap variable",
		"--card-pad variable",
		"--accent-dark variable",
		"fade-in constant",
		"--angle variable",
	}, []string{"color", "opacity", "syntax", "margin", "base", "--unit", "card", "root"})
}
