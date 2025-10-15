package services

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	shellsftp "github.com/charlesng35/shellcn/internal/sftp"
	"github.com/stretchr/testify/require"
)

type stubSFTPProvider struct {
	client     shellsftp.Client
	acquireErr error
	releaseErr error
	acquired   int
	released   int
	forceNil   bool
}

func (s *stubSFTPProvider) AcquireSFTP() (shellsftp.Client, func() error, error) {
	s.acquired++
	if s.acquireErr != nil {
		return nil, nil, s.acquireErr
	}
	if s.forceNil {
		return nil, func() error {
			s.released++
			return s.releaseErr
		}, nil
	}
	client := s.client
	if client == nil {
		client = &stubSFTPClient{}
	}
	return client, func() error {
		s.released++
		return s.releaseErr
	}, nil
}

type stubSFTPClient struct {
	readDir func(string) ([]os.FileInfo, error)
	stat    func(string) (os.FileInfo, error)
	open    func(string) (io.ReadCloser, error)
}

func (s *stubSFTPClient) ReadDir(path string) ([]os.FileInfo, error) {
	if s == nil || s.readDir == nil {
		return nil, nil
	}
	return s.readDir(path)
}

func (s *stubSFTPClient) Stat(path string) (os.FileInfo, error) {
	if s == nil || s.stat == nil {
		return nil, nil
	}
	return s.stat(path)
}

func (s *stubSFTPClient) Open(path string) (io.ReadCloser, error) {
	if s == nil || s.open == nil {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	return s.open(path)
}

func TestSFTPChannelService_AttachAndBorrow(t *testing.T) {
	svc := NewSFTPChannelService()
	provider := &stubSFTPProvider{}

	require.NoError(t, svc.Attach("session-1", provider))
	require.True(t, svc.Has("session-1"))

	client, release, err := svc.Borrow("session-1")
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, release)
	require.Equal(t, 1, provider.acquired)

	require.NoError(t, release())
	require.Equal(t, 1, provider.released)
}

func TestSFTPChannelService_BorrowMissing(t *testing.T) {
	svc := NewSFTPChannelService()

	client, release, err := svc.Borrow("missing")
	require.ErrorIs(t, err, ErrSFTPSessionNotFound)
	require.Nil(t, client)
	require.Nil(t, release)
}

func TestSFTPChannelService_DuplicateAttach(t *testing.T) {
	svc := NewSFTPChannelService()
	provider := &stubSFTPProvider{}

	require.NoError(t, svc.Attach("dup", provider))
	err := svc.Attach("dup", &stubSFTPProvider{})
	require.Error(t, err)
}

func TestSFTPChannelService_ProviderAcquireError(t *testing.T) {
	svc := NewSFTPChannelService()
	provider := &stubSFTPProvider{acquireErr: errors.New("fail")}

	require.NoError(t, svc.Attach("session", provider))

	client, release, err := svc.Borrow("session")
	require.Error(t, err)
	require.Nil(t, client)
	require.Nil(t, release)
	require.Equal(t, 1, provider.acquired)
}

func TestSFTPChannelService_ProviderReturnsNilClient(t *testing.T) {
	svc := NewSFTPChannelService()
	provider := &stubSFTPProvider{forceNil: true}

	require.NoError(t, svc.Attach("session", provider))

	client, release, err := svc.Borrow("session")
	require.ErrorIs(t, err, ErrSFTPProviderInvalid)
	require.Nil(t, client)
	require.Nil(t, release)
	require.Equal(t, 1, provider.released)
}
