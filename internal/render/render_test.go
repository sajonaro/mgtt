package render_test

import (
	"strings"
	"testing"

	"mgtt/internal/model"
	"mgtt/internal/render"
)

// ---------------------------------------------------------------------------
// TestCheckmark
// ---------------------------------------------------------------------------

func TestCheckmark(t *testing.T) {
	if got := render.Checkmark(true); got != "✓" {
		t.Errorf("Checkmark(true) = %q, want ✓", got)
	}
	if got := render.Checkmark(false); got != "✗" {
		t.Errorf("Checkmark(false) = %q, want ✗", got)
	}
}

// ---------------------------------------------------------------------------
// TestPluralize
// ---------------------------------------------------------------------------

func TestPluralize(t *testing.T) {
	cases := []struct {
		n        int
		singular string
		plural   string
		want     string
	}{
		{0, "error", "errors", "0 errors"},
		{1, "error", "errors", "1 error"},
		{2, "error", "errors", "2 errors"},
		{1, "warning", "warnings", "1 warning"},
		{4, "component", "components", "4 components"},
	}
	for _, c := range cases {
		got := render.Pluralize(c.n, c.singular, c.plural)
		if got != c.want {
			t.Errorf("Pluralize(%d, %q, %q) = %q, want %q", c.n, c.singular, c.plural, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestModelValidate_AllValid — 4 components, no errors
// ---------------------------------------------------------------------------

func TestModelValidate_AllValid(t *testing.T) {
	result := &model.ValidationResult{}
	components := []string{"nginx", "frontend", "api", "rds"}
	depCounts := map[string]int{
		"nginx":    2,
		"frontend": 1,
		"api":      1,
		"rds":      -1, // has healthy override, no deps
	}

	var buf strings.Builder
	render.ModelValidate(&buf, result, components, depCounts)
	out := buf.String()

	for _, name := range components {
		if !strings.Contains(out, name) {
			t.Errorf("output missing component name %q\n%s", name, out)
		}
	}
	if !strings.Contains(out, "0 errors") {
		t.Errorf("output missing '0 errors'\n%s", out)
	}
	if !strings.Contains(out, "4 components") {
		t.Errorf("output missing '4 components'\n%s", out)
	}
	if !strings.Contains(out, "healthy override valid") {
		t.Errorf("output missing 'healthy override valid'\n%s", out)
	}
	if strings.Contains(out, "✗") {
		t.Errorf("output should have no ✗ marks when all valid\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// TestModelValidate_WithErrors — error with "did you mean" suggestion
// ---------------------------------------------------------------------------

func TestModelValidate_WithErrors(t *testing.T) {
	result := &model.ValidationResult{
		Errors: []model.ValidationError{
			{
				Component:  "nginx",
				Field:      "depends",
				Message:    `dependency "nonexistent" references unknown component`,
				Suggestion: "api",
			},
		},
	}
	components := []string{"nginx"}
	depCounts := map[string]int{"nginx": 0}

	var buf strings.Builder
	render.ModelValidate(&buf, result, components, depCounts)
	out := buf.String()

	if !strings.Contains(out, "1 error") {
		t.Errorf("output missing '1 error'\n%s", out)
	}
	if !strings.Contains(out, "did you mean") {
		t.Errorf("output missing 'did you mean'\n%s", out)
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("output missing ✗ mark for error\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// TestModelValidate_HealthyOverride — no deps but has HealthyRaw → shows "healthy override valid"
// ---------------------------------------------------------------------------

func TestModelValidate_HealthyOverride(t *testing.T) {
	result := &model.ValidationResult{}
	components := []string{"rds"}
	// -1 signals: no deps, but has healthy override
	depCounts := map[string]int{"rds": -1}

	var buf strings.Builder
	render.ModelValidate(&buf, result, components, depCounts)
	out := buf.String()

	if !strings.Contains(out, "healthy override valid") {
		t.Errorf("output missing 'healthy override valid'\n%s", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("output missing ✓ mark\n%s", out)
	}
}
