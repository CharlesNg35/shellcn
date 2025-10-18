# Role Template System - Explanation & Design Rationale

**Date:** 2025-01-19
**Status:** Implementation Started (based on code changes)

---

## Why Role Templates? The Core Problem

### **Problem: Role Mutation Side Effects**

Without templates, roles are **shared objects** that can be assigned to multiple teams/users:

```
Role: "Developer"
  ├─> Assigned to: User Alice
  ├─> Assigned to: Team Engineering
  └─> Assigned to: Team QA
```

**Scenario:**
1. Team Engineering needs to add `protocol:ssh.port_forward` permission
2. Admin modifies the "Developer" role to add the permission
3. **Side Effect:** User Alice and Team QA now also have port forwarding access (unintended privilege escalation)

**Result:** You cannot customize permissions per team without affecting everyone using that role.

---

## Solution: Template vs Instance Pattern

### **Design: Separate Templates from Instances**

```
Role Template: "Developer" (is_template=true, id="template-developer")
  ├─> Defines BASE permissions: [connection.view, connection.launch]
  │
  ├─> Team Engineering Instance (template_id="template-developer")
  │   └─> Inherits base + adds: [protocol:ssh.port_forward]
  │
  ├─> Team QA Instance (template_id="template-developer")
  │   └─> Inherits base + adds: [vault.view]
  │
  └─> User Alice Instance (template_id="template-developer")
      └─> Inherits base only
```

**Key Benefit:** Modifying one instance does NOT affect other instances.

---

## How It Works (Based on Your GORM Implementation)

### **1. Role Model (Updated)**

```go
// internal/models/role.go
type Role struct {
    BaseModel

    Name        string  `gorm:"uniqueIndex;not null"`
    Description string
    IsSystem    bool    `gorm:"default:false"`

    // ↓ Template fields
    IsTemplate  bool    `gorm:"default:false;index"`
    TemplateID  *string `gorm:"type:uuid;index"`  // ← Points to template role ID

    Permissions []Permission `gorm:"many2many:role_permissions;"`
    Users       []User       `gorm:"many2many:user_roles;"`
}
```

**Two Types of Roles:**
1. **Template Role:** `is_template=true`, `template_id=nil`
   - Blueprint/prototype
   - Cannot be assigned directly to users/teams
   - Defines base permission set

2. **Instance Role:** `is_template=false`, `template_id="<template-id>"`
   - Instantiated from a template
   - Can be assigned to teams/users
   - Inherits template permissions + custom additions

---

### **2. Seeding Templates (Database Initialization)**

```go
// internal/database/migrations.go (line 44-59)
roles := []models.Role{
    {
        BaseModel:   models.BaseModel{ID: "admin"},
        Name:        "Administrator",
        Description: "Full system access",
        IsSystem:    true,
        IsTemplate:  true,  // ← Template
    },
    {
        BaseModel:   models.BaseModel{ID: "user"},
        Name:        "User",
        Description: "Standard user access",
        IsSystem:    true,
        IsTemplate:  true,  // ← Template
    },
}
```

**System creates base templates on first run:**
- `admin` template (full permissions)
- `user` template (standard permissions)

---

### **3. Creating a Team - Role Assignment Workflow**

#### **Option A: Assign Existing Template Instance**

```go
// internal/services/team_service.go
func (s *TeamService) Create(ctx context.Context, input CreateTeamInput) (*models.Team, error) {
    team := &models.Team{
        Name:        input.Name,
        Description: input.Description,
    }

    if err := s.db.WithContext(ctx).Create(team).Error; err != nil {
        return nil, err
    }

    // Option A: Assign existing "user" template directly
    if input.DefaultRole {
        var userTemplate models.Role
        if err := s.db.WithContext(ctx).
            First(&userTemplate, "id = ? AND is_template = ?", "user", true).Error; err != nil {
            return nil, err
        }

        // Assign template to team (shared role)
        if err := s.db.Model(team).Association("Roles").Append(&userTemplate); err != nil {
            return nil, err
        }
    }

    return team, nil
}
```

**Problem with Option A:** Template is shared → mutation affects all teams using it (same issue as before).

---

#### **Option B: Create Team-Specific Instance (Recommended)**

```go
// internal/services/team_service.go (PROPOSED ADDITION)

func (s *TeamService) Create(ctx context.Context, input CreateTeamInput) (*models.Team, error) {
    return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // 1. Create team
        team := &models.Team{
            Name:        input.Name,
            Description: input.Description,
        }

        if err := tx.Create(team).Error; err != nil {
            return err
        }

        // 2. Instantiate role from template
        if input.TemplateRoleID != "" {
            var template models.Role
            if err := tx.Preload("Permissions").
                First(&template, "id = ? AND is_template = ?", input.TemplateRoleID, true).Error; err != nil {
                return fmt.Errorf("template not found: %w", err)
            }

            // Create team-specific instance
            instance := models.Role{
                Name:        fmt.Sprintf("%s (%s)", template.Name, team.Name),
                Description: fmt.Sprintf("Team-specific role for %s", team.Name),
                IsTemplate:  false,           // ← Not a template
                TemplateID:  &template.ID,    // ← Points to template
            }

            if err := tx.Create(&instance).Error; err != nil {
                return err
            }

            // Copy permissions from template
            if err := tx.Model(&instance).Association("Permissions").
                Replace(template.Permissions); err != nil {
                return err
            }

            // Assign instance to team
            if err := tx.Model(team).Association("Roles").Append(&instance); err != nil {
                return err
            }
        }

        return nil
    })
}
```

**Benefits:**
- ✅ Each team gets its own role instance
- ✅ Modifications to Team A's role don't affect Team B
- ✅ Template updates can optionally cascade to instances (admin choice)

---

## Complete Workflow Example

### **Step 1: System Initialization (Automatic)**

```sql
-- Seeded during database setup
INSERT INTO roles (id, name, is_template, is_system)
VALUES
    ('template-admin', 'Administrator Template', true, true),
    ('template-developer', 'Developer Template', true, true),
    ('template-viewer', 'Viewer Template', true, true);

-- Assign base permissions to templates
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'template-developer', p.id
FROM permissions p
WHERE p.id IN ('connection.view', 'connection.launch', 'protocol:ssh.connect');
```

---

### **Step 2: Create Team "Engineering"**

**Admin Action:** Create team with "Developer Template"

```go
// POST /api/teams
{
    "name": "Engineering",
    "description": "Engineering team",
    "template_role_id": "template-developer"  // ← Choose template
}
```

**Backend Processing:**
```go
// 1. Create team
team := Team{Name: "Engineering"}
db.Create(&team)

// 2. Load template
template := Role{} // is_template=true, id=template-developer
db.Preload("Permissions").First(&template, "id = ?", "template-developer")

// 3. Create team-specific instance
instance := Role{
    Name:        "Developer (Engineering)",  // ← Unique name
    IsTemplate:  false,
    TemplateID:  "template-developer",       // ← Reference
}
db.Create(&instance)

// 4. Copy template permissions to instance
db.Model(&instance).Association("Permissions").Replace(template.Permissions)
// Result: instance has [connection.view, connection.launch, protocol:ssh.connect]

// 5. Assign instance to team
db.Model(&team).Association("Roles").Append(&instance)
```

**Database State:**
```sql
-- roles table
id: role-eng-123
name: "Developer (Engineering)"
is_template: false
template_id: template-developer

-- role_permissions (copied from template)
role-eng-123 | connection.view
role-eng-123 | connection.launch
role-eng-123 | protocol:ssh.connect

-- team_roles (assigned to team)
team: engineering-456 | role: role-eng-123
```

---

### **Step 3: Customize Team Engineering's Permissions**

**Admin Action:** Add port forwarding permission to Engineering team (without affecting QA team)

```go
// PATCH /api/roles/role-eng-123/permissions
{
    "permission_ids": [
        "connection.view",
        "connection.launch",
        "protocol:ssh.connect",
        "protocol:ssh.port_forward"  // ← Add new
    ]
}
```

**Backend:**
```go
// Update role-eng-123 permissions (this is an instance, not template)
db.Model(&instance).Association("Permissions").Replace(newPermissions)
```

**Result:**
- ✅ Team Engineering now has port forwarding
- ✅ Team QA (using different instance) still has base permissions only
- ✅ Template unchanged (other teams can still instantiate it)

---

### **Step 4: Create Team "QA" (Same Template, Different Permissions)**

```go
// POST /api/teams
{
    "name": "QA",
    "template_role_id": "template-developer"  // ← Same template
}
```

**Backend creates new instance:**
```sql
-- New role instance for QA
id: role-qa-789
name: "Developer (QA)"
is_template: false
template_id: template-developer

-- Permissions copied from template (base set only)
role-qa-789 | connection.view
role-qa-789 | connection.launch
role-qa-789 | protocol:ssh.connect
```

**Result:**
- ✅ Team QA has base developer permissions
- ✅ Team QA does NOT have port forwarding (only Engineering has it)
- ✅ Teams are isolated

---

## Why This Design is Accurate

### **1. Industry Standard Pattern**

This is the **"Role Prototype"** pattern used by:
- **Kubernetes:** RoleBinding vs ClusterRole (templates)
- **AWS IAM:** Managed Policies (templates) vs Customer Policies (instances)
- **Azure RBAC:** Built-in roles (templates) vs Custom roles (instances)

### **2. GORM-Native Implementation**

```go
// Self-referential foreign key (standard GORM pattern)
type Role struct {
    BaseModel
    TemplateID *string `gorm:"type:uuid;index"`  // ← Points to roles.id
}
```

No special GORM features required. Works with all databases (PostgreSQL, MySQL, SQLite).

### **3. Solves Mutation Problem**

**Without Templates:**
```
Modify "Developer" role → Affects ALL users/teams using it
```

**With Templates:**
```
Modify "Developer (Engineering)" instance → Affects ONLY Engineering team
```

### **4. Maintains Template Consistency**

If you update a template:
```sql
-- Update template
UPDATE role_permissions
SET ...
WHERE role_id = 'template-developer';

-- Optionally cascade to instances (admin choice)
UPDATE role_permissions rp
SET rp.permission_id = ...
WHERE rp.role_id IN (
    SELECT id FROM roles WHERE template_id = 'template-developer'
);
```

---

## Alternative Considered: Why Not Just Create Unique Roles?

**Alternative:** Don't use templates, just create unique roles per team:
```
Role: "Engineering Developer"
Role: "QA Developer"
Role: "Marketing Developer"
```

**Problems:**
1. **Role Explosion:** 100 teams × 5 role types = 500 roles
2. **No Consistency:** Hard to ensure base permissions are correct
3. **No Template Updates:** Can't apply security patches to all "developer" roles at once
4. **Naming Chaos:** "Engineering Developer" vs "Developer (Engineering)" vs "Eng-Dev"

**Template Solution Avoids These Problems:**
- ✅ Single template definition
- ✅ Automatic naming: `{template.Name} ({team.Name})`
- ✅ Optional cascade updates
- ✅ Clear lineage tracking via `template_id`

---

## Implementation Status (Based on Code Changes)

### ✅ **Completed:**
1. Role model updated with `is_template`, `template_id` fields
2. Permission model enhanced with `display_name`, `category`, `default_scope`
3. ResourcePermission model created with polymorphic support
4. Seed data creates templates (`admin`, `user`)
5. Permission registry supports protocol-specific permissions
6. PermissionService supports template creation/updates

### 🚧 **TODO:**
1. **TeamService.Create()** - Add template instantiation logic (Option B above)
2. **API Endpoint** - `POST /api/teams` accept `template_role_id` parameter
3. **Frontend** - Team creation form shows template dropdown
4. **RoleService** - Add `InstantiateTemplate(templateID, teamID)` method
5. **Migration** - Backfill existing team roles into instances

---

## Recommended Team Creation API

```go
// internal/handlers/team_handler.go (PROPOSED)

type createTeamPayload struct {
    Name           string  `json:"name" binding:"required"`
    Description    string  `json:"description"`
    TemplateRoleID *string `json:"template_role_id"`  // ← Optional template
}

func (h *TeamHandler) Create(c *gin.Context) {
    var payload createTeamPayload
    if !bindAndValidate(c, &payload) {
        return
    }

    userID := c.GetString(middleware.CtxUserIDKey)

    team, err := h.svc.CreateWithTemplate(requestContext(c), services.CreateTeamInput{
        Name:           payload.Name,
        Description:    payload.Description,
        TemplateRoleID: payload.TemplateRoleID,  // ← Pass template ID
        CreatedBy:      userID,
    })

    if err != nil {
        response.Error(c, err)
        return
    }

    response.Success(c, http.StatusCreated, team)
}
```

**Frontend Team Creation Form:**
```typescript
// POST /api/teams
{
    "name": "Engineering",
    "description": "Engineering team",
    "template_role_id": "template-developer"  // ← Dropdown selection
}

// Template Dropdown Options (from GET /api/roles?is_template=true):
[
    { id: "template-admin", name: "Administrator Template", description: "Full access" },
    { id: "template-developer", name: "Developer Template", description: "Standard dev access" },
    { id: "template-viewer", name: "Viewer Template", description: "Read-only" }
]
```

---

## Summary

**Is Template Accurate?** ✅ **YES**

**How Team Role Assignment Works:**
1. Admin creates team → selects template (e.g., "Developer Template")
2. System creates **team-specific role instance** from template
3. Instance inherits template permissions (can be customized)
4. Team gets its own role → modifications don't affect other teams
5. Template updates can optionally propagate to instances

**Key Insight:** Templates are **blueprints**, instances are **copies**. This prevents the "shared role mutation" problem while maintaining consistency.

---

**Next Step:** Implement `TeamService.CreateWithTemplate()` method to automate instance creation during team setup.
