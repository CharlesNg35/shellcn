package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/charlesng35/shellcn/pkg/crypto"
)

// PKCEPair represents the verifier/challenge material required for PKCE flows.
type PKCEPair struct {
	Verifier  string
	Challenge string
}

// GeneratePKCE produces a PKCE verifier and associated S256 challenge.
func GeneratePKCE() (PKCEPair, error) {
	verifier, err := crypto.GenerateToken(64)
	if err != nil {
		return PKCEPair{}, fmt.Errorf("pkce: generate verifier: %w", err)
	}

	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	return PKCEPair{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}
