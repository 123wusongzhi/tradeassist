package ozon

import platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"

// RegisterProvider registers the Ozon Seller API platform provider.
func RegisterProvider() {
	platformp.Register(NewProvider())
}
