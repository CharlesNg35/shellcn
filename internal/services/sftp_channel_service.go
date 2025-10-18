package services

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	shellsftp "github.com/charlesng35/shellcn/internal/sftp"
)

// SFTPProvider exposes pooled SFTP clients for an active SSH session.
type SFTPProvider interface {
	AcquireSFTP() (shellsftp.Client, func() error, error)
}

var (
	// ErrSFTPSessionNotFound is returned when no SFTP provider is registered for the session.
	ErrSFTPSessionNotFound = errors.New("sftp channel service: session not registered")
	// ErrSFTPProviderInvalid indicates an invalid or missing provider was supplied.
	ErrSFTPProviderInvalid = errors.New("sftp channel service: provider is invalid")
)

// SFTPChannelService tracks active session → SFTP provider bindings.
type SFTPChannelService struct {
	mu        sync.RWMutex
	providers map[string]SFTPProvider
}

// NewSFTPChannelService constructs an empty channel registry.
func NewSFTPChannelService() *SFTPChannelService {
	return &SFTPChannelService{
		providers: make(map[string]SFTPProvider),
	}
}

// Attach registers a provider for the supplied session identifier.
func (s *SFTPChannelService) Attach(sessionID string, provider SFTPProvider) error {
	if s == nil {
		return fmt.Errorf("%w: service not initialised", ErrSFTPProviderInvalid)
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || provider == nil {
		return ErrSFTPProviderInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.providers[sessionID]; exists {
		return fmt.Errorf("sftp channel service: session %s already registered", sessionID)
	}

	s.providers[sessionID] = provider
	return nil
}

// Detach removes the provider associated with the session, if any.
func (s *SFTPChannelService) Detach(sessionID string) {
	if s == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	s.mu.Lock()
	delete(s.providers, sessionID)
	s.mu.Unlock()
}

// Borrow returns a pooled SFTP client for the registered session.
// Callers must invoke the returned release function once done.
func (s *SFTPChannelService) Borrow(sessionID string) (shellsftp.Client, func() error, error) {
	if s == nil {
		return nil, nil, ErrSFTPProviderInvalid
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, nil, ErrSFTPProviderInvalid
	}

	s.mu.RLock()
	provider, ok := s.providers[sessionID]
	s.mu.RUnlock()
	if !ok || provider == nil {
		return nil, nil, ErrSFTPSessionNotFound
	}

	client, release, err := provider.AcquireSFTP()
	if err != nil {
		return nil, nil, err
	}
	if client == nil {
		if release != nil {
			_ = release()
		}
		return nil, nil, ErrSFTPProviderInvalid
	}
	if release == nil {
		return nil, nil, ErrSFTPProviderInvalid
	}
	return client, release, nil
}

// Has returns true when a provider is registered for the session.
func (s *SFTPChannelService) Has(sessionID string) bool {
	if s == nil {
		return false
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false
	}

	s.mu.RLock()
	_, ok := s.providers[sessionID]
	s.mu.RUnlock()
	return ok
}
