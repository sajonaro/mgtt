package providersupport

import (
	"strings"
	"testing"
)

func TestCheckRequires_NilOK(t *testing.T) {
	if err := CheckRequires(nil); err != nil {
		t.Fatalf("nil requires must be ok, got %v", err)
	}
}

func TestCheckRequires_UnknownKeyIgnored(t *testing.T) {
	if err := CheckRequires(map[string]string{"redis": ">=5.0.0"}); err != nil {
		t.Fatalf("unknown require key should be ignored, got %v", err)
	}
}

func TestCheckRequires_CompatibleMgtt(t *testing.T) {
	if err := CheckRequires(map[string]string{"mgtt": ">=0.0.1"}); err != nil {
		t.Fatalf("0.0.1 should satisfy current MgttVersion, got %v", err)
	}
}

func TestCheckRequires_IncompatibleMgtt(t *testing.T) {
	err := CheckRequires(map[string]string{"mgtt": ">=99.0.0"})
	if err == nil {
		t.Fatal("99.0.0 should fail")
	}
	if !strings.Contains(err.Error(), "requires mgtt") {
		t.Fatalf("error should explain mismatch: %v", err)
	}
}

func TestCheckRequires_RejectsNonGTE(t *testing.T) {
	cases := []string{"^1.0.0", "~1.0", ">=1.0 <2.0", "1.0.0", ""}
	for _, c := range cases {
		err := CheckRequires(map[string]string{"mgtt": c})
		if err == nil {
			t.Errorf("constraint %q should be rejected", c)
		}
	}
}

func TestProvider_CheckCompatible_BypassByCallers(t *testing.T) {
	// Loaders parse Requires but do not gate. Demonstrates the contract:
	// callers (use vs. uninstall) decide whether to call CheckCompatible.
	yamlSrc := []byte(`
meta:
  name: x
  version: 1.0.0
  requires:
    mgtt: ">=99.0.0"
  command: "/tmp/x"
auth:
  strategy: environment
  access:
    probes: read
    writes: none
`)
	p, err := LoadFromBytes(yamlSrc)
	if err != nil {
		t.Fatalf("Load should succeed regardless of compat (uninstall path): %v", err)
	}
	if err := p.CheckCompatible(); err == nil {
		t.Fatal("CheckCompatible should fail for incompatible provider")
	}
}
