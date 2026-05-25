package models

import "time"

// PolicyRule is an additive role grant loaded into the embedded Casbin enforcer.
type PolicyRule struct {
	ID         string    `gorm:"primaryKey"`
	Role       Role      `gorm:"index:idx_policy_rule,unique"`
	Permission string    `gorm:"index:idx_policy_rule,unique"`
	Risk       string    `gorm:"index:idx_policy_rule,unique"`
	CreatedAt  time.Time `gorm:"index"`
}

func (PolicyRule) TableName() string { return "policy_rules" }
