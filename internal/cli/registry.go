package cli

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mgt-tool/mgtt/internal/providersupport"
	"github.com/mgt-tool/mgtt/internal/providersupport/genericprovider"
)

// debugEnabled reports whether MGTT_DEBUG is set to a truthy value. The
// registry helpers gate their observational DEBUG lines on this to keep
// normal stdout/stderr clean unless operators opt in.
func debugEnabled() bool {
	v := strings.TrimSpace(os.Getenv("MGTT_DEBUG"))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// reservedGenericNameError formats the error emitted when an on-disk
// provider claims the reserved "generic" name. Callers at registry-load
// and install-time paths share this wording.
func reservedGenericNameError(sources []string) error {
	return fmt.Errorf("provider(s) %v declare meta.name=%q, which is reserved for mgtt's built-in generic fallback — rename them or uninstall",
		sources, providersupport.GenericProviderName)
}

// loadRegistryForUse loads every discovered provider via LoadAllForUse and
// registers the embedded generic fallback provider. Callers that want
// strict type-resolution (no fallback) should use providersupport
// directly — this helper is the "normal" path used by plan / diagnose /
// simulate / status / model validate.
func loadRegistryForUse() (*providersupport.Registry, error) {
	reg, reserved := providersupport.LoadAllForUse()
	if len(reserved) > 0 {
		return nil, reservedGenericNameError(reserved)
	}
	if err := genericprovider.Register(reg); err != nil {
		return nil, fmt.Errorf("register generic provider: %w", err)
	}
	if debugEnabled() {
		log.Printf("[registry] registered built-in generic fallback provider")
	}
	return reg, nil
}

// loadRegistryAll mirrors loadRegistryForUse but uses LoadAllEmbedded
// instead (no compatibility filter). Used by `mgtt ls` and similar
// discovery paths that must still see incompatible providers.
func loadRegistryAll() (*providersupport.Registry, error) {
	reg, reserved := providersupport.LoadAllEmbedded()
	if len(reserved) > 0 {
		return nil, reservedGenericNameError(reserved)
	}
	if err := genericprovider.Register(reg); err != nil {
		return nil, fmt.Errorf("register generic provider: %w", err)
	}
	if debugEnabled() {
		log.Printf("[registry] registered built-in generic fallback provider")
	}
	return reg, nil
}
