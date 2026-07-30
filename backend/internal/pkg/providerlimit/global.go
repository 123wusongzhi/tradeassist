package providerlimit

import "sync"

var (
	globalOnce sync.Once
	globalReg  *Registry
)

// Global returns the shared provider concurrency registry.
func Global() *Registry {
	globalOnce.Do(func() {
		globalReg = NewRegistry(Config{
			DefaultConcurrency: 8,
			ProviderOverrides: map[ProviderName]int{
				ProviderDouyinShop: 16,
				ProviderAI:         8,
				ProviderImage:      6,
				ProviderStorage:    8,
				ProviderSecurity:   4,
			},
			OperationOverrides: map[ProviderOperation]int{
				OperationTokenRefresh: 4,
				OperationDraftWrite:   4,
				OperationText:         8,
				OperationImage:        6,
				OperationObject:       8,
			},
			MaxEntries: 128,
		})
	})
	return globalReg
}

// SetGlobalForTest replaces the shared registry in tests.
func SetGlobalForTest(reg *Registry) {
	globalReg = reg
	globalOnce = sync.Once{}
	if reg != nil {
		globalOnce.Do(func() {})
	}
}
