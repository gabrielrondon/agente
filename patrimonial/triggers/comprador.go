package triggers

import (
	"fmt"
	"strings"
)

// CompradorTrigger formats a message to send to the Comprador agent.
// In a full integration, this would call the Comprador binary or an internal API.
type CompradorTrigger struct{}

// NewCompradorTrigger creates a CompradorTrigger.
func NewCompradorTrigger() *CompradorTrigger {
	return &CompradorTrigger{}
}

// TriggerResult describes what was sent to the Comprador.
type TriggerResult struct {
	Command     string
	Description string
}

// Buy triggers the Comprador to quote items needed for an asset.
func (t *CompradorTrigger) Buy(assetName string, items []string) (*TriggerResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("nenhum item para comprar")
	}

	description := fmt.Sprintf("Para %s: %s", assetName, strings.Join(items, ", "))
	cmd := fmt.Sprintf("comprador quote %q", description)

	fmt.Println("\n=== Acionar Comprador ===")
	fmt.Printf("Execute o comando abaixo para solicitar cotação:\n\n  %s\n\n", cmd)

	return &TriggerResult{
		Command:     cmd,
		Description: description,
	}, nil
}
