package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
)

func TestSnippetService_CreateAndList(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	svc, err := NewSnippetService(db)
	require.NoError(t, err)

	ctx := context.Background()

	created, err := svc.Create(ctx, CreateSnippetInput{
		Name:        "List files",
		Description: "List home directory contents",
		Command:     "ls -la",
		Scope:       "user",
		OwnerUserID: "user-1",
	})
	require.NoError(t, err)
	require.Equal(t, "user", created.Scope)
	require.NotEmpty(t, created.ID)

	snippets, err := svc.List(ctx, ListSnippetsOptions{
		OwnerUserID: "user-1",
		IncludeUser: true,
		Scope:       "all",
	})
	require.NoError(t, err)
	require.Len(t, snippets, 1)
	require.Equal(t, "List files", snippets[0].Name)
}
