// Package models holds the core entity types shared across ShellCN. These
// structs double as the GORM models (gorm tags live directly on them); only
// internal/store imports the gorm package, so the ORM never leaks outward.
package models
