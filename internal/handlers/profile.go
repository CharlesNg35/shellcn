package handlers

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/auth/mfa"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/crypto"
	appErrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// ProfileHandler exposes current-user account management endpoints.
type ProfileHandler struct {
	users *services.UserService
	prefs *services.UserPreferencesService
	totp  *mfa.TOTPService
}

// NewProfileHandler configures a profile handler with required services.
func NewProfileHandler(users *services.UserService, prefs *services.UserPreferencesService, totp *mfa.TOTPService) *ProfileHandler {
	return &ProfileHandler{users: users, prefs: prefs, totp: totp}
}

type updateProfileRequest struct {
	Username  *string `json:"username" validate:"omitempty,min=3,max=64"`
	Email     *string `json:"email" validate:"omitempty,email"`
	FirstName *string `json:"first_name" validate:"omitempty,max=128"`
	LastName  *string `json:"last_name" validate:"omitempty,max=128"`
	Avatar    *string `json:"avatar" validate:"omitempty,max=512"`
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,min=8"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

type mfaCodeRequest struct {
	Code string `json:"code" validate:"required,min=6,max=8"`
}

type mfaSetupResponse struct {
	Secret     string   `json:"secret"`
	OTPAuthURL string   `json:"otpauth_url"`
	QRCode     string   `json:"qr_code"`
	Backup     []string `json:"backup_codes"`
}

type updatePreferencesRequest struct {
	SSH sshPreferencesRequest `json:"ssh" validate:"required"`
}

type sshPreferencesRequest struct {
	Terminal sshTerminalPreferencesRequest `json:"terminal" validate:"required"`
	SFTP     sshSFTPPreferencesRequest     `json:"sftp" validate:"required"`
}

type sshTerminalPreferencesRequest struct {
	FontFamily   string `json:"font_family" validate:"omitempty,max=128"`
	CursorStyle  string `json:"cursor_style" validate:"omitempty,oneof=block underline beam bar line vertical"`
	CopyOnSelect bool   `json:"copy_on_select"`
}

type sshSFTPPreferencesRequest struct {
	ShowHiddenFiles bool `json:"show_hidden_files"`
	AutoOpenQueue   bool `json:"auto_open_queue"`
}

// Update modifies the authenticated user's profile details.
func (h *ProfileHandler) Update(c *gin.Context) {
	var body updateProfileRequest
	if !bindAndValidate(c, &body) {
		return
	}

	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	userID, _ := v.(string)

	input := services.UpdateUserInput{
		Username:  body.Username,
		Email:     body.Email,
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Avatar:    body.Avatar,
	}

	user, err := h.users.Update(requestContext(c), userID, input)
	if err != nil {
		respondUserError(c, err, "updated")
		return
	}

	response.Success(c, http.StatusOK, marshalProfileUser(user))
}

// GetPreferences returns the current user's preference profile.
func (h *ProfileHandler) GetPreferences(c *gin.Context) {
	if h.prefs == nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	userID, _ := v.(string)

	prefs, err := h.prefs.Get(requestContext(c), userID)
	if err != nil {
		respondUserError(c, err, "retrieved preferences")
		return
	}

	response.Success(c, http.StatusOK, prefs)
}

// UpdatePreferences applies updated preference values for the current user.
func (h *ProfileHandler) UpdatePreferences(c *gin.Context) {
	if h.prefs == nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	var body updatePreferencesRequest
	if !bindAndValidate(c, &body) {
		return
	}

	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	userID, _ := v.(string)

	payload := services.UserPreferences{
		SSH: services.SSHPreferences{
			Terminal: services.SSHTerminalPreferences{
				FontFamily:   strings.TrimSpace(body.SSH.Terminal.FontFamily),
				CursorStyle:  body.SSH.Terminal.CursorStyle,
				CopyOnSelect: body.SSH.Terminal.CopyOnSelect,
			},
			SFTP: services.SSHSFTPPreferences{
				ShowHiddenFiles: body.SSH.SFTP.ShowHiddenFiles,
				AutoOpenQueue:   body.SSH.SFTP.AutoOpenQueue,
			},
		},
	}

	updated, err := h.prefs.Update(requestContext(c), userID, payload)
	if err != nil {
		respondUserError(c, err, "updated preferences")
		return
	}

	response.Success(c, http.StatusOK, updated)
}

// ChangePassword updates the authenticated user's password after verifying the current value.
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	var body passwordChangeRequest
	if !bindAndValidate(c, &body) {
		return
	}

	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	userID, _ := v.(string)

	if strings.TrimSpace(body.CurrentPassword) == strings.TrimSpace(body.NewPassword) {
		response.Error(c, appErrors.NewBadRequest("new password must differ from the current password"))
		return
	}

	user, err := h.users.GetByID(requestContext(c), userID)
	if err != nil {
		respondUserError(c, err, "updated")
		return
	}

	if provider := strings.ToLower(strings.TrimSpace(user.AuthProvider)); provider != "" && provider != "local" {
		response.Error(c, appErrors.NewBadRequest("passwords are managed by the external identity provider"))
		return
	}

	if !crypto.VerifyPassword(user.Password, body.CurrentPassword) {
		response.Error(c, appErrors.NewBadRequest("current password is incorrect"))
		return
	}

	if err := h.users.ChangePassword(requestContext(c), userID, body.NewPassword); err != nil {
		respondUserError(c, err, "updated")
		return
	}

	response.Success(c, http.StatusOK, gin.H{"updated": true})
}

// SetupMFA provisions a temporary secret and backup codes for MFA enrollment.
func (h *ProfileHandler) SetupMFA(c *gin.Context) {
	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	userID, _ := v.(string)

	user, err := h.users.GetByID(requestContext(c), userID)
	if err != nil {
		respondUserError(c, err, "retrieved")
		return
	}

	key, backupCodes, err := h.totp.GenerateSecret(user.ID, user.Username)
	if err != nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	qr, err := h.totp.GenerateQRCode(key)
	if err != nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	payload := mfaSetupResponse{
		Secret:     key.Secret(),
		OTPAuthURL: key.URL(),
		QRCode:     base64.StdEncoding.EncodeToString(qr),
		Backup:     backupCodes,
	}

	response.Success(c, http.StatusOK, payload)
}

// EnableMFA verifies a TOTP code and activates MFA for the account.
func (h *ProfileHandler) EnableMFA(c *gin.Context) {
	var body mfaCodeRequest
	if !bindAndValidate(c, &body) {
		return
	}

	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	userID, _ := v.(string)

	valid, err := h.totp.VerifyCode(userID, body.Code)
	if err != nil {
		response.Error(c, appErrors.NewBadRequest("verification failed"))
		return
	}
	if !valid {
		response.Error(c, appErrors.NewBadRequest("verification code is invalid"))
		return
	}

	if err := h.users.SetMFAEnabled(requestContext(c), userID, true); err != nil {
		respondUserError(c, err, "updated")
		return
	}

	response.Success(c, http.StatusOK, gin.H{"enabled": true})
}

// DisableMFA removes the MFA secret after validating the provided code.
func (h *ProfileHandler) DisableMFA(c *gin.Context) {
	var body mfaCodeRequest
	if !bindAndValidate(c, &body) {
		return
	}

	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}
	userID, _ := v.(string)

	valid, err := h.totp.VerifyCode(userID, body.Code)
	if err != nil {
		response.Error(c, appErrors.NewBadRequest("verification failed"))
		return
	}
	if !valid {
		response.Error(c, appErrors.NewBadRequest("verification code is invalid"))
		return
	}

	if err := h.totp.DeleteSecret(userID); err != nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	if err := h.users.SetMFAEnabled(requestContext(c), userID, false); err != nil {
		respondUserError(c, err, "updated")
		return
	}

	response.Success(c, http.StatusOK, gin.H{"disabled": true})
}

func marshalProfileUser(user *models.User) gin.H {
	if user == nil {
		return gin.H{}
	}

	return gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"first_name":    user.FirstName,
		"last_name":     user.LastName,
		"avatar":        strings.TrimSpace(user.Avatar),
		"is_root":       user.IsRoot,
		"is_active":     user.IsActive,
		"auth_provider": user.AuthProvider,
		"mfa_enabled":   user.MFAEnabled,
	}
}
