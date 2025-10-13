package handlers

import (
	"context"
	stdErrors "errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/mfa"
	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/monitoring"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// AuthHandler manages authentication flows (login/refresh/logout/me).
type AuthHandler struct {
	db        *gorm.DB
	jwt       *iauth.JWTService
	sessions  *iauth.SessionService
	providers *services.AuthProviderService
	sso       *iauth.SSOManager
	ldapSync  *services.LDAPSyncService
	totp      *mfa.TOTPService
	verifier  *services.EmailVerificationService
}

func NewAuthHandler(db *gorm.DB, jwt *iauth.JWTService, sessions *iauth.SessionService, providers *services.AuthProviderService, sso *iauth.SSOManager, ldapSync *services.LDAPSyncService, totp *mfa.TOTPService, verifier *services.EmailVerificationService) *AuthHandler {
	return &AuthHandler{db: db, jwt: jwt, sessions: sessions, providers: providers, sso: sso, ldapSync: ldapSync, totp: totp, verifier: verifier}
}

type loginRequest struct {
	Identifier string `json:"identifier" validate:"required"`
	Password   string `json:"password" validate:"required"`
	Provider   string `json:"provider"`
}

type verifyMfaRequest struct {
	ChallengeID    string `json:"challenge_id" validate:"required"`
	MFAToken       string `json:"mfa_token" validate:"required"`
	RememberDevice bool   `json:"remember_device"`
}

type registerRequest struct {
	Username  string `json:"username" validate:"required,min=3,max=64"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8,max=128"`
	FirstName string `json:"first_name" validate:"max=64"`
	LastName  string `json:"last_name" validate:"max=64"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if !bindAndValidate(c, &req) {
		return
	}
	req.Identifier = strings.TrimSpace(req.Identifier)
	if req.Identifier == "" {
		response.Error(c, errors.NewBadRequest("identifier is required"))
		return
	}

	providerType := strings.ToLower(strings.TrimSpace(req.Provider))
	if providerType == "" || providerType == "local" {
		h.handleLocalLogin(c, req)
		return
	}

	switch providerType {
	case "ldap":
		h.handleLDAPLogin(c, req)
	default:
		response.Error(c, errors.ErrBadRequest)
	}
}

// POST /api/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if !bindAndValidate(c, &req) {
		return
	}

	lp, err := providers.NewLocalProvider(h.db, providers.LocalConfig{})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	if h.verifier != nil {
		lp.SetEmailVerifier(h.verifier)
	}

	user, err := lp.Register(providers.RegisterInput{
		Username:  req.Username,
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		switch {
		case stdErrors.Is(err, providers.ErrRegistrationDisabled):
			response.Error(c, errors.ErrForbidden)
		case stdErrors.Is(err, providers.ErrVerificationUnavailable):
			response.Error(c, errors.ErrInternalServer)
		default:
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
				response.Error(c, errors.NewBadRequest("username or email already exists"))
			} else if strings.Contains(strings.ToLower(err.Error()), "not found") {
				response.Error(c, errors.ErrBadRequest)
			} else {
				response.Error(c, errors.ErrInternalServer)
			}
		}
		return
	}

	requiresVerification := !user.IsActive
	message := "Account created. You can now sign in."
	if requiresVerification {
		message = "Account created. Check your email to verify your account before signing in."
	}

	response.Success(c, http.StatusCreated, gin.H{
		"registered":            true,
		"requires_verification": requiresVerification,
		"message":               message,
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// POST /api/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if !bindAndValidate(c, &req) {
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		response.Error(c, errors.NewBadRequest("refresh token is required"))
		return
	}

	pair, _, err := h.sessions.RefreshSession(req.RefreshToken)
	if err != nil {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	expiresIn := int(h.jwt.AccessTokenTTL().Seconds())
	if expiresIn <= 0 {
		expiresIn = int(iauth.DefaultAccessTokenTTL.Seconds())
	}

	response.Success(c, http.StatusOK, tokenResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresIn: expiresIn})
}

// POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	v, ok := c.Get("sessionID")
	if !ok {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	sid, _ := v.(string)
	if sid == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	if err := h.sessions.RevokeSession(sid); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"revoked": true})
}

// GET /api/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	userID, _ := v.(string)

	var user models.User
	if err := h.db.Preload("Teams").Preload("Roles").Take(&user, "id = ?", userID).Error; err != nil {
		response.Error(c, errors.ErrNotFound)
		return
	}

	ctx := requestContext(c)
	checker, _ := permissions.NewChecker(h.db)
	perms, _ := checker.GetUserPermissions(ctx, user.ID)

	provider := normalizeProvider(user.AuthProvider)
	emailVerified := true
	if requires, err := h.requiresEmailVerification(ctx, provider); err == nil {
		if requires {
			if verified, verr := h.isEmailVerified(ctx, user.ID); verr == nil {
				emailVerified = verified
			} else {
				response.Error(c, errors.ErrInternalServer)
				return
			}
		}
	} else {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	payload := gin.H{
		"id":             user.ID,
		"username":       user.Username,
		"email":          user.Email,
		"is_root":        user.IsRoot,
		"is_active":      user.IsActive,
		"first_name":     user.FirstName,
		"last_name":      user.LastName,
		"avatar":         strings.TrimSpace(user.Avatar),
		"auth_provider":  provider,
		"mfa_enabled":    user.MFAEnabled,
		"email_verified": emailVerified,
		"permissions":    perms,
	}

	response.Success(c, http.StatusOK, payload)
}

func (h *AuthHandler) handleLocalLogin(c *gin.Context, req loginRequest) {
	lp, err := providers.NewLocalProvider(h.db, providers.LocalConfig{})
	if err != nil {
		monitoring.RecordAuthAttempt("failure")
		response.Error(c, errors.ErrInternalServer)
		return
	}

	user, err := lp.Authenticate(providers.AuthenticateInput{
		Identifier: req.Identifier,
		Password:   req.Password,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	})
	if err != nil {
		monitoring.RecordAuthAttempt("failure")
		response.Error(c, errors.ErrInvalidCredentials)
		return
	}

	if user.MFAEnabled {
		if h.totp == nil {
			monitoring.RecordAuthAttempt("failure")
			response.Error(c, errors.ErrInternalServer)
			return
		}
		challengeID, ttl, err := h.sessions.CreateMFAChallenge(user.ID, iauth.SessionMetadata{
			IPAddress: c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			Claims: map[string]any{
				"username": user.Username,
				"email":    user.Email,
			},
		})
		if err != nil {
			monitoring.RecordAuthAttempt("failure")
			response.Error(c, errors.ErrInternalServer)
			return
		}

		expires := int(ttl.Seconds())
		if expires < 0 {
			expires = 0
		}
		expiresAt := time.Now().Add(ttl).UTC()

		challenge := gin.H{
			"id":         challengeID,
			"method":     "totp",
			"expires_in": expires,
			"expires_at": expiresAt.Format(time.RFC3339),
		}
		c.JSON(http.StatusUnauthorized, response.Response{
			Success: false,
			Error: &response.ErrorInfo{
				Code:    errors.ErrMFARequired.Code,
				Message: errors.ErrMFARequired.Message,
				Details: map[string]any{"challenge": challenge},
			},
			Data: gin.H{
				"challenge": challenge,
			},
		})
		return
	}

	pair, _, err := h.sessions.CreateSession(user.ID, iauth.SessionMetadata{
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Claims: map[string]any{
			"username": user.Username,
			"email":    user.Email,
		},
	})
	if err != nil {
		monitoring.RecordAuthAttempt("failure")
		response.Error(c, errors.ErrInternalServer)
		return
	}

	monitoring.RecordAuthAttempt("success")
	h.respondWithTokens(c, user, pair)
}

func (h *AuthHandler) handleLDAPLogin(c *gin.Context, req loginRequest) {
	if h.providers == nil || h.sso == nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	providerModel, cfg, err := h.providers.LoadLDAPConfig(requestContext(c))
	if err != nil || !providerModel.Enabled {
		monitoring.RecordAuthAttempt("failure")
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	authenticator, err := providers.NewLDAPAuthenticator(*cfg, providers.LDAPAuthenticatorOptions{})
	if err != nil {
		monitoring.RecordAuthAttempt("failure")
		response.Error(c, errors.ErrInternalServer)
		return
	}

	identity, err := authenticator.Authenticate(requestContext(c), providers.LDAPAuthenticateInput{
		Identifier: req.Identifier,
		Password:   req.Password,
	})
	if err != nil {
		monitoring.RecordAuthAttempt("failure")
		response.Error(c, errors.ErrInvalidCredentials)
		return
	}

	tokens, user, session, err := h.sso.Resolve(requestContext(c), *identity, iauth.ResolveOptions{
		AutoProvision: providerModel.AllowRegistration,
		SessionMeta: iauth.SessionMetadata{
			IPAddress: c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		},
	})
	if err != nil {
		monitoring.RecordAuthAttempt("failure")
		switch {
		case stdErrors.Is(err, iauth.ErrSSOUserNotFound),
			stdErrors.Is(err, iauth.ErrSSOProviderMismatch),
			stdErrors.Is(err, iauth.ErrSSOUserDisabled),
			stdErrors.Is(err, iauth.ErrSSOEmailRequired):
			response.Error(c, errors.ErrInvalidCredentials)
		default:
			response.Error(c, errors.ErrUnauthorized)
		}
		return
	}

	if h.ldapSync != nil && cfg.SyncGroups {
		if _, syncErr := h.ldapSync.SyncGroups(requestContext(c), *cfg, user, identity.Groups); syncErr != nil {
			monitoring.RecordAuthAttempt("failure")
			if session != nil {
				_ = h.sessions.RevokeSession(session.ID)
			}
			response.Error(c, errors.ErrInternalServer)
			return
		}
	}

	monitoring.RecordAuthAttempt("success")
	h.respondWithTokens(c, user, tokens)
}

// POST /api/auth/mfa/verify
func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	if h.sessions == nil || h.totp == nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	var req verifyMfaRequest
	if !bindAndValidate(c, &req) {
		return
	}

	userID, meta, err := h.sessions.ConsumeMFAChallenge(req.ChallengeID)
	if err != nil {
		switch {
		case stdErrors.Is(err, iauth.ErrMFAChallengeExpired), stdErrors.Is(err, iauth.ErrMFAChallengeNotFound):
			monitoring.RecordAuthAttempt("failure")
			response.Error(c, errors.ErrMFAInvalid)
		default:
			response.Error(c, errors.ErrInternalServer)
		}
		return
	}

	valid, verifyErr := h.totp.VerifyCode(userID, req.MFAToken)
	if verifyErr != nil {
		if stdErrors.Is(verifyErr, mfa.ErrSecretNotFound) {
			monitoring.RecordAuthAttempt("failure")
			response.Error(c, errors.ErrMFAInvalid)
			return
		}
		monitoring.RecordAuthAttempt("failure")
		response.Error(c, errors.ErrInternalServer)
		return
	}
	if !valid {
		monitoring.RecordAuthAttempt("failure")
		response.Error(c, errors.ErrMFAInvalid)
		return
	}

	var user models.User
	if err := h.db.Take(&user, "id = ?", userID).Error; err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	meta.IPAddress = c.ClientIP()
	meta.UserAgent = c.Request.UserAgent()
	if meta.Claims == nil {
		meta.Claims = make(map[string]any)
	}
	meta.Claims["username"] = user.Username
	meta.Claims["email"] = user.Email
	if req.RememberDevice {
		meta.Claims["mfa_remember"] = true
	}

	pair, _, err := h.sessions.CreateSession(userID, meta)
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	monitoring.RecordAuthAttempt("success")
	h.respondWithTokens(c, &user, pair)
}

// POST /api/auth/email/resend
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	if h.verifier == nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	userID, _ := v.(string)

	ctx := requestContext(c)

	var user models.User
	if err := h.db.Take(&user, "id = ?", userID).Error; err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	provider := normalizeProvider(user.AuthProvider)
	requires, err := h.requiresEmailVerification(ctx, provider)
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	if !requires {
		response.Success(c, http.StatusOK, gin.H{"sent": false})
		return
	}

	verified, err := h.isEmailVerified(ctx, user.ID)
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	if verified {
		response.Success(c, http.StatusOK, gin.H{"sent": false})
		return
	}

	if _, _, err := h.verifier.CreateToken(ctx, user.ID, user.Email); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"sent": true})
}

func normalizeProvider(provider string) string {
	value := strings.ToLower(strings.TrimSpace(provider))
	if value == "" {
		return "local"
	}
	return value
}

func (h *AuthHandler) requiresEmailVerification(ctx context.Context, provider string) (bool, error) {
	if normalizeProvider(provider) != "local" {
		return false, nil
	}

	var record models.AuthProvider
	if err := h.db.WithContext(ctx).
		Select("require_email_verification").
		Where("type = ?", "local").
		First(&record).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		if strings.Contains(err.Error(), "no such table") {
			return false, nil
		}
		return false, err
	}

	return record.RequireEmailVerification, nil
}

func (h *AuthHandler) isEmailVerified(ctx context.Context, userID string) (bool, error) {
	var verification models.EmailVerification
	if err := h.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&verification).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil
		}
		if strings.Contains(err.Error(), "no such table") {
			return true, nil
		}
		return false, err
	}

	return verification.VerifiedAt != nil, nil
}

func (h *AuthHandler) respondWithTokens(c *gin.Context, user *models.User, pair iauth.TokenPair) {
	// Reload user to include associations required by the response payload.
	var hydrated models.User
	if err := h.db.Preload("Roles").Take(&hydrated, "id = ?", user.ID).Error; err == nil {
		user = &hydrated
	}

	ctx := requestContext(c)

	checker, _ := permissions.NewChecker(h.db)
	perms, _ := checker.GetUserPermissions(ctx, user.ID)

	provider := normalizeProvider(user.AuthProvider)

	emailVerified := true
	if requires, err := h.requiresEmailVerification(ctx, provider); err == nil {
		if requires {
			if verified, verr := h.isEmailVerified(ctx, user.ID); verr == nil {
				emailVerified = verified
			} else {
				response.Error(c, errors.ErrInternalServer)
				return
			}
		}
	} else {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	roles := make([]gin.H, 0, len(user.Roles))
	for _, role := range user.Roles {
		roles = append(roles, gin.H{
			"id":          role.ID,
			"name":        role.Name,
			"description": role.Description,
		})
	}

	expiresIn := int(h.jwt.AccessTokenTTL().Seconds())
	if expiresIn <= 0 {
		expiresIn = int(iauth.DefaultAccessTokenTTL.Seconds())
	}

	payload := gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    expiresIn,
		"user": gin.H{
			"id":             user.ID,
			"username":       user.Username,
			"email":          user.Email,
			"is_root":        user.IsRoot,
			"is_active":      user.IsActive,
			"first_name":     user.FirstName,
			"last_name":      user.LastName,
			"roles":          roles,
			"permissions":    perms,
			"auth_provider":  provider,
			"mfa_enabled":    user.MFAEnabled,
			"email_verified": emailVerified,
		},
	}

	response.Success(c, http.StatusOK, payload)
}
