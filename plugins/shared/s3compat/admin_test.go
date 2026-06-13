package s3compat

import (
	"errors"
	"testing"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestValidateBucketName(t *testing.T) {
	valid := []string{"my-bucket", "data.bucket", "a1b", "backups-2026"}
	for _, name := range valid {
		if got, err := validateBucketName(name); err != nil || got != name {
			t.Errorf("validateBucketName(%q) = %q, %v; want valid", name, got, err)
		}
	}
	invalid := []string{"", "  ", "ab", "-leading", "trailing-", "UPPER", "under_score", "has space", "double..dot", "with/slash"}
	for _, name := range invalid {
		if _, err := validateBucketName(name); !errors.Is(err, plugin.ErrInvalidInput) {
			t.Errorf("validateBucketName(%q) err = %v; want ErrInvalidInput", name, err)
		}
	}
}

func TestValidateVersioningStatus(t *testing.T) {
	cases := map[string]types.BucketVersioningStatus{
		"Enabled":   types.BucketVersioningStatusEnabled,
		"enabled":   types.BucketVersioningStatusEnabled,
		"Suspended": types.BucketVersioningStatusSuspended,
		"SUSPENDED": types.BucketVersioningStatusSuspended,
	}
	for in, want := range cases {
		got, err := validateVersioningStatus(in)
		if err != nil || got != want {
			t.Errorf("validateVersioningStatus(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "disabled", "on", "off"} {
		if _, err := validateVersioningStatus(in); !errors.Is(err, plugin.ErrInvalidInput) {
			t.Errorf("validateVersioningStatus(%q) err = %v; want ErrInvalidInput", in, err)
		}
	}
}

func TestParseExpiry(t *testing.T) {
	if d, err := parseExpiry(""); err != nil || d != defaultPresignExpiry {
		t.Errorf("blank expiry = %v, %v; want default %v", d, err, defaultPresignExpiry)
	}
	if d, err := parseExpiry("300"); err != nil || d != 300*time.Second {
		t.Errorf("expiry 300 = %v, %v; want 5m", d, err)
	}
	// Clamped to the SigV4 ceiling.
	if d, err := parseExpiry("99999999"); err != nil || d != maxPresignExpiry {
		t.Errorf("oversized expiry = %v, %v; want %v", d, err, maxPresignExpiry)
	}
	for _, in := range []string{"0", "-5", "abc", "1.5"} {
		if _, err := parseExpiry(in); !errors.Is(err, plugin.ErrInvalidInput) {
			t.Errorf("parseExpiry(%q) err = %v; want ErrInvalidInput", in, err)
		}
	}
}

func TestPresignKey(t *testing.T) {
	c := &Client{bucket: "b", prefix: "team/"}
	if key, err := presignKey(c, "/docs/report.pdf"); err != nil || key != "team/docs/report.pdf" {
		t.Errorf("presignKey = %q, %v; want team/docs/report.pdf", key, err)
	}
	for _, in := range []string{"", ".", "/", "dir/"} {
		if _, err := presignKey(c, in); !errors.Is(err, plugin.ErrInvalidInput) {
			t.Errorf("presignKey(%q) err = %v; want ErrInvalidInput", in, err)
		}
	}
}

func TestCreateBucketInput(t *testing.T) {
	if in := createBucketInput("b", "us-east-1"); in.CreateBucketConfiguration != nil {
		t.Error("us-east-1 must omit a location constraint")
	}
	in := createBucketInput("b", "eu-west-1")
	if in.CreateBucketConfiguration == nil ||
		in.CreateBucketConfiguration.LocationConstraint != types.BucketLocationConstraint("eu-west-1") {
		t.Errorf("non-default region must set a matching location constraint: %+v", in.CreateBucketConfiguration)
	}
	if in := createBucketInput("b", ""); in.CreateBucketConfiguration != nil {
		t.Error("empty region must omit a location constraint")
	}
}

type apiErr struct{ code string }

func (e apiErr) Error() string                 { return e.code }
func (e apiErr) ErrorCode() string             { return e.code }
func (e apiErr) ErrorMessage() string          { return e.code }
func (e apiErr) ErrorFault() smithy.ErrorFault { return smithy.FaultServer }

func TestMapAdminError(t *testing.T) {
	cases := map[string]error{
		"NoSuchBucket":            plugin.ErrNotFound,
		"AccessDenied":            plugin.ErrForbidden,
		"BucketAlreadyOwnedByYou": plugin.ErrConflict,
		"BucketNotEmpty":          plugin.ErrConflict,
		"InvalidBucketName":       plugin.ErrInvalidInput,
		"SomethingElse":           plugin.ErrUnavailable,
	}
	for code, want := range cases {
		got := mapAdminError(apiErr{code: code})
		if !errors.Is(got, want) {
			t.Errorf("mapAdminError(%q) = %v; want %v", code, got, want)
		}
	}
	if mapAdminError(nil) != nil {
		t.Error("mapAdminError(nil) must be nil")
	}
}

func TestAdminRoutesMetadata(t *testing.T) {
	routes := AdminRoutes("minio")
	byID := map[string]plugin.Route{}
	for _, r := range routes {
		byID[r.ID] = r
	}
	if r, ok := byID["minio.bucket.delete"]; !ok || r.Risk != plugin.RiskDestructive {
		t.Errorf("bucket.delete must be destructive: %+v", r)
	}
	if r := byID["minio.bucket.create"]; r.Risk != plugin.RiskWrite || r.Input == nil {
		t.Errorf("bucket.create must be write with an input schema: %+v", r)
	}
	if r := byID["minio.bucket.versioning.set"]; r.Risk != plugin.RiskWrite || r.Input == nil {
		t.Errorf("versioning.set must be write with an input schema: %+v", r)
	}
	if r := byID["minio.object.presign"]; r.Risk != plugin.RiskSafe {
		t.Errorf("presign must be safe: %+v", r)
	}
	for _, r := range routes {
		if r.Permission == "" || r.AuditEvent == "" {
			t.Errorf("route %q missing permission/audit", r.ID)
		}
	}
}

func TestAdminManifestUX(t *testing.T) {
	tab := BucketTab("minio")
	cfg, ok := tab.Config.(plugin.TableConfig)
	if !ok {
		t.Fatalf("bucket tab config = %T", tab.Config)
	}
	if cfg.EmptyText == "" || cfg.RowClick != plugin.RowClickDetail || cfg.DefaultSort == nil || cfg.DefaultSort.Field != "createdAt" {
		t.Fatalf("bucket tab should declare empty text and row detail behavior: %#v", cfg)
	}
	if !contains(cfg.RowActionIDs, "minio.bucket.versions") {
		t.Fatalf("bucket tab should expose versions inspector action: %#v", cfg.RowActionIDs)
	}

	actions := Actions("minio")
	byID := map[string]plugin.Action{}
	for _, action := range actions {
		byID[action.ID] = action
	}
	versions := byID["minio.bucket.versions"]
	if versions.Open != plugin.OpenDialog || versions.Panel != plugin.PanelTable {
		t.Fatalf("versions action should open a table dialog: %+v", versions)
	}
	versionsCfg, ok := versions.Config.(plugin.TableConfig)
	if !ok || versionsCfg.DefaultSort == nil || versionsCfg.DefaultSort.Field != "modTime" || !versionsCfg.Exportable {
		t.Fatalf("versions action should declare an exportable typed table: %#v", versions.Config)
	}
	if a := byID["minio.bucket.versioning.set"]; !a.Confirm || a.Label != "Edit versioning" {
		t.Fatalf("versioning action should be explicit and confirmed: %+v", a)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestBucketCreateSchemaValidatesPortableNames(t *testing.T) {
	schema := bucketCreateSchema()
	field := schema.Groups[0].Fields[0]
	if field.Key != "name" || len(field.Validators) == 0 {
		t.Fatalf("bucket name field missing validator: %+v", field)
	}
	if field.Validators[0].Type != plugin.ValidatorRegex || field.Validators[0].Message == "" {
		t.Fatalf("bucket name validator should be a user-facing regex: %+v", field.Validators[0])
	}
	region := schema.Groups[0].Fields[1]
	if region.Key != "region" || region.Type != plugin.FieldAutocomplete || !region.Required || region.Default != "us-east-1" {
		t.Fatalf("bucket region should be required autocomplete with AWS default: %+v", region)
	}
}

// ensure the SDK paginator constructor exists for the version we build against.
var _ = awss3.NewListObjectVersionsPaginator
