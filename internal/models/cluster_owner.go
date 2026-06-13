package models

import "time"

type ClusterOwner struct {
	Key          string `gorm:"primaryKey;column:owner_key"`
	InstanceID   string `gorm:"index;not null"`
	InternalURL  string
	InternalURLs string
	LeaseID      string    `gorm:"index;not null"`
	ExpiresAt    time.Time `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (ClusterOwner) TableName() string { return "cluster_owners" }
