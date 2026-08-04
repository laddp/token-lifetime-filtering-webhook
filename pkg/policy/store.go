package policy

import (
	"log/slog"
	"sync"

	tokenv1alpha1 "github.com/pladd/token-lifetime-filtering-webhook/api/v1alpha1"
)

// Store holds the singleton TokenLifetimePolicy/cluster in memory.
type Store struct {
	mu     sync.RWMutex
	policy *tokenv1alpha1.TokenLifetimePolicy
}

// NewStore returns an empty policy store.
func NewStore() *Store {
	return &Store{}
}

// Get returns a deep copy of the current singleton policy, or nil if unset.
func (s *Store) Get() *tokenv1alpha1.TokenLifetimePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.policy == nil {
		return nil
	}
	return s.policy.DeepCopy()
}

// Upsert stores policy if it is the cluster singleton; otherwise logs and ignores.
func (s *Store) Upsert(policy *tokenv1alpha1.TokenLifetimePolicy) {
	if policy == nil {
		return
	}
	if policy.Name != tokenv1alpha1.SingletonName {
		slog.Warn("ignoring TokenLifetimePolicy; only cluster singleton is honored",
			"name", policy.Name,
			"expected", tokenv1alpha1.SingletonName,
		)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = policy.DeepCopy()
}

// Delete clears the store if name is the singleton.
func (s *Store) Delete(name string) {
	if name != tokenv1alpha1.SingletonName {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = nil
}
