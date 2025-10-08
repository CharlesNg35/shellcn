package models

import "testing"

func TestBaseModelBeforeCreateGeneratesID(t *testing.T) {
	var base BaseModel
	if err := base.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if base.ID == "" {
		t.Fatal("expected base model ID to be generated")
	}
}

func TestEmbeddedModelsUseBaseBeforeCreate(t *testing.T) {
	cases := []struct {
		name  string
		model func() *BaseModel
	}{
		{"user", func() *BaseModel {
			u := &User{}
			return &u.BaseModel
		}},
		{"organization", func() *BaseModel {
			o := &Organization{}
			return &o.BaseModel
		}},
		{"team", func() *BaseModel {
			m := &Team{}
			return &m.BaseModel
		}},
		{"role", func() *BaseModel {
			r := &Role{}
			return &r.BaseModel
		}},
		{"permission", func() *BaseModel {
			p := &Permission{}
			return &p.BaseModel
		}},
		{"session", func() *BaseModel {
			s := &Session{}
			return &s.BaseModel
		}},
		{"audit_log", func() *BaseModel {
			a := &AuditLog{}
			return &a.BaseModel
		}},
		{"mfa_secret", func() *BaseModel {
			m := &MFASecret{}
			return &m.BaseModel
		}},
		{"password_reset_token", func() *BaseModel {
			p := &PasswordResetToken{}
			return &p.BaseModel
		}},
		{"auth_provider", func() *BaseModel {
			a := &AuthProvider{}
			return &a.BaseModel
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := tc.model()
			if err := model.BeforeCreate(nil); err != nil {
				t.Fatalf("before create: %v", err)
			}
			if model.ID == "" {
				t.Fatal("expected ID to be generated")
			}
		})
	}
}
