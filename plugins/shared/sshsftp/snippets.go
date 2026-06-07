package sshsftp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const snippetStorageNamespace = "snippets"

type storedSnippet struct {
	ID        string
	Name      string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type snippetStore struct {
	storage plugin.Storage
}

func newSnippetStore(storage plugin.Storage) *snippetStore {
	if storage == nil {
		return nil
	}
	return &snippetStore{storage: storage}
}

func (s *snippetStore) Create(ctx context.Context, sn *storedSnippet) error {
	item, err := snippetToStorageItem(*sn)
	if err != nil {
		return err
	}
	stored, err := s.storage.Put(ctx, item)
	if err != nil {
		return err
	}
	*sn = snippetFromStorageItem(stored)
	return nil
}

func (s *snippetStore) Get(ctx context.Context, id string) (storedSnippet, error) {
	item, err := s.storage.Get(ctx, snippetStorageScope(), id)
	if err != nil {
		return storedSnippet{}, err
	}
	return snippetFromStorageItem(item), nil
}

func (s *snippetStore) List(ctx context.Context) ([]storedSnippet, error) {
	rows, err := s.storage.List(ctx, snippetStorageScope(), "")
	if err != nil {
		return nil, err
	}
	out := make([]storedSnippet, 0, len(rows))
	for _, row := range rows {
		out = append(out, snippetFromStorageItem(row))
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (s *snippetStore) Delete(ctx context.Context, id string) error {
	return s.storage.Delete(ctx, snippetStorageScope(), id)
}

func snippetStorageScope() plugin.StorageScope {
	return plugin.StorageScope{
		Namespace:  snippetStorageNamespace,
		UserScoped: true,
	}
}

type snippetValue struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

func snippetToStorageItem(sn storedSnippet) (plugin.StorageItem, error) {
	body, err := json.Marshal(snippetValue{Name: sn.Name, Body: sn.Body})
	if err != nil {
		return plugin.StorageItem{}, err
	}
	return plugin.StorageItem{
		Scope:       snippetStorageScope(),
		Key:         sn.ID,
		Value:       body,
		ContentType: "application/vnd.shellcn.snippet+json",
		Metadata:    map[string]string{"name": sn.Name},
		CreatedAt:   sn.CreatedAt,
		UpdatedAt:   sn.UpdatedAt,
	}, nil
}

func snippetFromStorageItem(item plugin.StorageItem) storedSnippet {
	var value snippetValue
	_ = json.Unmarshal(item.Value, &value)
	if value.Name == "" {
		value.Name = item.Metadata["name"]
	}
	return storedSnippet{
		ID:        item.Key,
		Name:      value.Name,
		Body:      value.Body,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}
