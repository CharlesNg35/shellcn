package store

import (
	"encoding/json"

	"github.com/charlesng35/shellcn/internal/models"
)

func preferredInternalURLForClaim(current, next models.LiveStateLease) string {
	if current.InternalURL == "" {
		return next.InternalURL
	}
	if current.InternalURL == next.InternalURL {
		return current.InternalURL
	}
	var candidates []string
	if err := json.Unmarshal([]byte(next.InternalURLs), &candidates); err != nil {
		return next.InternalURL
	}
	for _, candidate := range candidates {
		if candidate == current.InternalURL {
			return current.InternalURL
		}
	}
	return next.InternalURL
}
