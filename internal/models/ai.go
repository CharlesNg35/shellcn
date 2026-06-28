package models

import "time"

// AIProviderKind selects the engine adapter. Custom providers are just
// openai_compatible rows with their own name and base URL.
type AIProviderKind string

const (
	AIProviderOpenAI       AIProviderKind = "openai"
	AIProviderOpenRouter   AIProviderKind = "openrouter"
	AIProviderAnthropic    AIProviderKind = "anthropic"
	AIProviderGoogle       AIProviderKind = "google"
	AIProviderOpenAICompat AIProviderKind = "openai_compatible"
)

// AIProviderConfig is a user-scoped AI provider the owner manages themselves.
// The API key is stored only as ciphertext (encrypted above the store via the
// Vault) and never serializes to clients. Global/shared AI is config-only and
// has no row here.
type AIProviderConfig struct {
	ID      string         `gorm:"primaryKey"`
	OwnerID string         `gorm:"index;uniqueIndex:idx_ai_provider_owner_name"`
	Kind    AIProviderKind `gorm:"index"`
	Name    string         `gorm:"uniqueIndex:idx_ai_provider_owner_name"`
	BaseURL string
	Models  []string `gorm:"serializer:json"`
	Model   string
	// APIKeyCiphertext is opaque ciphertext; the store never sees the plaintext key.
	APIKeyCiphertext []byte `json:"-"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (AIProviderConfig) TableName() string { return "ai_provider_configs" }

// AIProviderSummary is the non-secret projection returned to clients: it never
// includes the key, only whether one is set.
type AIProviderSummary struct {
	ID        string         `json:"id"`
	Kind      AIProviderKind `json:"kind"`
	Name      string         `json:"name"`
	BaseURL   string         `json:"baseUrl,omitempty"`
	Models    []string       `json:"models"`
	Model     string         `json:"model"`
	HasKey    bool           `json:"hasKey"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// AIConversation is one chat thread, scoped to a user + connection. Summary holds
// the rolling compaction of older turns kept within the model's context window.
type AIConversation struct {
	ID           string `gorm:"primaryKey" json:"id"`
	OwnerID      string `gorm:"index" json:"ownerId"`
	ConnectionID string `gorm:"index" json:"connectionId"`
	Title        string `json:"title"`
	AutoTitled   bool   `json:"autoTitled"`
	// ProviderID is the user provider used (empty = shared/global). Model records
	// which model served the thread.
	ProviderID string `json:"providerId"`
	Model      string `json:"model"`
	// Summary is the rolling compaction of older turns (see internal/ai/memory).
	Summary   string    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (AIConversation) TableName() string { return "ai_conversations" }

// AIToolCallRecord is a persisted tool call/result pair on an assistant message.
type AIToolCallRecord struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Input  any    `json:"input,omitempty"`
	Output any    `json:"output,omitempty"`
	Err    string `json:"err,omitempty"`
}

// AIMessage is one persisted turn message. ToolCalls capture the assistant's tool
// activity; Reasoning is optional model thinking.
type AIMessage struct {
	ID             string             `gorm:"primaryKey" json:"id"`
	ConversationID string             `gorm:"index;uniqueIndex:idx_ai_messages_conversation_seq" json:"conversationId"`
	Seq            int                `gorm:"index;uniqueIndex:idx_ai_messages_conversation_seq" json:"seq"`
	Role           string             `json:"role"` // user | assistant
	Content        string             `json:"content"`
	ToolCalls      []AIToolCallRecord `gorm:"serializer:json" json:"toolCalls"`
	Reasoning      string             `json:"reasoning,omitempty"`
	Truncated      bool               `json:"truncated,omitempty"`
	CreatedAt      time.Time          `json:"createdAt"`
}

func (AIMessage) TableName() string { return "ai_messages" }

// Summary projects the row to its non-secret client form.
func (c AIProviderConfig) Summary() AIProviderSummary {
	return AIProviderSummary{
		ID:        c.ID,
		Kind:      c.Kind,
		Name:      c.Name,
		BaseURL:   c.BaseURL,
		Models:    c.Models,
		Model:     c.Model,
		HasKey:    len(c.APIKeyCiphertext) > 0,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}
