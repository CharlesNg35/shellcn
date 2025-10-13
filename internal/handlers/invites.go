package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
	appErrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type InviteHandler struct {
	invites  *services.InviteService
	users    *services.UserService
	teams    *services.TeamService
	verifier *services.EmailVerificationService
}

func NewInviteHandler(
	invites *services.InviteService,
	users *services.UserService,
	teams *services.TeamService,
	verifier *services.EmailVerificationService,
) *InviteHandler {
	return &InviteHandler{
		invites:  invites,
		users:    users,
		teams:    teams,
		verifier: verifier,
	}
}

type createInviteRequest struct {
	Email  string `json:"email" validate:"required,email"`
	TeamID string `json:"team_id" validate:"omitempty,uuid4"`
}

type redeemInviteRequest struct {
	Token     string `json:"token" validate:"required"`
	Username  string `json:"username" validate:"required,min=3,max=64"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"omitempty,max=128"`
	LastName  string `json:"last_name" validate:"omitempty,max=128"`
}

type inviteDTO struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	InvitedBy  string     `json:"invited_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	Status     string     `json:"status"`
	TeamID     string     `json:"team_id,omitempty"`
	TeamName   string     `json:"team_name,omitempty"`
}

type inviteCreatedResponse struct {
	Invite inviteDTO `json:"invite"`
	Token  string    `json:"token"`
	Link   string    `json:"link,omitempty"`
}

type redeemInviteResponse struct {
	User    inviteUserDTO `json:"user"`
	Message string        `json:"message"`
}

type inviteUserDTO struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	IsActive  bool   `json:"is_active"`
}

// POST /api/invites
func (h *InviteHandler) Create(c *gin.Context) {
	if h.invites == nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	var req createInviteRequest
	if !bindAndValidate(c, &req) {
		return
	}

	ctx := requestContext(c)

	teamID := strings.TrimSpace(req.TeamID)
	var team *models.Team
	if teamID != "" {
		if h.teams == nil {
			response.Error(c, appErrors.ErrInternalServer)
			return
		}
		var err error
		team, err = h.teams.GetByID(ctx, teamID, userID)
		if err != nil {
			response.Error(c, err)
			return
		}
	}

	invite, token, link, err := h.invites.GenerateInvite(ctx, req.Email, userID, teamID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInviteAlreadyPending):
			response.Error(c, appErrors.NewBadRequest("An active invite already exists for this email"))
		case errors.Is(err, services.ErrInviteEmailInUse):
			response.Error(c, appErrors.NewBadRequest("An account already exists for this email address"))
		case errors.Is(err, services.ErrInviteUserAlreadyInTeam):
			response.Error(c, appErrors.NewBadRequest("User is already a member of the selected team"))
		case errors.Is(err, services.ErrTeamNotFound):
			response.Error(c, appErrors.NewBadRequest("Selected team could not be found"))
		default:
			response.Error(c, appErrors.ErrInternalServer)
		}
		return
	}

	if team != nil {
		invite.Team = team
	}

	if strings.TrimSpace(link) == "" {
		link = "/invite/accept?token=" + url.QueryEscape(token)
	}

	payload := inviteCreatedResponse{
		Invite: toInviteDTO(invite, time.Now()),
		Token:  token,
		Link:   link,
	}

	response.Success(c, http.StatusCreated, payload)
}

// GET /api/invites
func (h *InviteHandler) List(c *gin.Context) {
	if h.invites == nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	search := c.Query("search")

	invites, err := h.invites.List(requestContext(c), status, search)
	if err != nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	now := time.Now()
	items := make([]inviteDTO, 0, len(invites))
	for i := range invites {
		items = append(items, toInviteDTO(&invites[i], now))
	}

	response.Success(c, http.StatusOK, gin.H{"invites": items})
}

// DELETE /api/invites/:id
func (h *InviteHandler) Delete(c *gin.Context) {
	if h.invites == nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	inviteID := c.Param("id")
	if strings.TrimSpace(inviteID) == "" {
		response.Error(c, appErrors.NewBadRequest("Invite ID is required"))
		return
	}

	if err := h.invites.Delete(requestContext(c), inviteID); err != nil {
		switch {
		case errors.Is(err, services.ErrInviteNotFound):
			response.Error(c, appErrors.ErrNotFound)
		case errors.Is(err, services.ErrInviteAlreadyUsed):
			response.Error(c, appErrors.NewBadRequest("Invite has already been accepted"))
		default:
			response.Error(c, appErrors.ErrInternalServer)
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

// POST /api/invites/:id/resend
func (h *InviteHandler) Resend(c *gin.Context) {
	if h.invites == nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	inviteID := strings.TrimSpace(c.Param("id"))
	if inviteID == "" {
		response.Error(c, appErrors.NewBadRequest("Invite ID is required"))
		return
	}

	invite, token, link, err := h.invites.ResendInvite(requestContext(c), inviteID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInviteNotFound):
			response.Error(c, appErrors.ErrNotFound)
		case errors.Is(err, services.ErrInviteAlreadyUsed):
			response.Error(c, appErrors.NewBadRequest("Invite has already been accepted"))
		default:
			response.Error(c, appErrors.ErrInternalServer)
		}
		return
	}

	payload := inviteCreatedResponse{
		Invite: toInviteDTO(invite, time.Now()),
		Token:  token,
		Link:   link,
	}
	response.Success(c, http.StatusOK, payload)
}

// POST /api/invites/:id/link
func (h *InviteHandler) IssueLink(c *gin.Context) {
	if h.invites == nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	inviteID := strings.TrimSpace(c.Param("id"))
	if inviteID == "" {
		response.Error(c, appErrors.NewBadRequest("Invite ID is required"))
		return
	}

	invite, token, link, err := h.invites.IssueInviteLink(requestContext(c), inviteID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInviteNotFound):
			response.Error(c, appErrors.ErrNotFound)
		case errors.Is(err, services.ErrInviteAlreadyUsed):
			response.Error(c, appErrors.NewBadRequest("Invite has already been accepted"))
		default:
			response.Error(c, appErrors.ErrInternalServer)
		}
		return
	}

	payload := inviteCreatedResponse{
		Invite: toInviteDTO(invite, time.Now()),
		Token:  token,
		Link:   link,
	}
	response.Success(c, http.StatusOK, payload)
}

// POST /api/auth/invites/redeem
func (h *InviteHandler) Redeem(c *gin.Context) {
	if h.invites == nil || h.users == nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	var req redeemInviteRequest
	if !bindAndValidate(c, &req) {
		return
	}

	ctx := requestContext(c)

	invite, err := h.invites.ValidateToken(ctx, req.Token)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInviteNotFound):
			response.Error(c, appErrors.NewBadRequest("Invite token is invalid"))
		case errors.Is(err, services.ErrInviteExpired):
			response.Error(c, appErrors.NewBadRequest("Invite token has expired"))
		case errors.Is(err, services.ErrInviteAlreadyUsed):
			response.Error(c, appErrors.NewBadRequest("Invite has already been used"))
		default:
			response.Error(c, appErrors.ErrInternalServer)
		}
		return
	}

	requiresVerification := false

	existingUser, err := h.users.FindByEmail(ctx, invite.Email)
	if err != nil {
		response.Error(c, err)
		return
	}

	createdUser := false
	user := existingUser
	isActive := !requiresVerification

	if user == nil {
		userInput := services.CreateUserInput{
			Username:  strings.TrimSpace(req.Username),
			Email:     strings.ToLower(strings.TrimSpace(invite.Email)),
			Password:  req.Password,
			FirstName: strings.TrimSpace(req.FirstName),
			LastName:  strings.TrimSpace(req.LastName),
			IsActive:  &isActive,
		}

		user, err = h.users.Create(ctx, userInput)
		if err != nil {
			response.Error(c, err)
			return
		}
		createdUser = true

		if requiresVerification && h.verifier != nil {
			if _, _, err := h.verifier.CreateToken(ctx, user.ID, user.Email); err != nil {
				_ = h.users.Delete(ctx, user.ID)
				response.Error(c, appErrors.ErrInternalServer)
				return
			}
		}
	}

	addedToTeam := false
	if invite.TeamID != nil && h.teams != nil {
		err := h.teams.AddMember(ctx, *invite.TeamID, user.ID)
		if err != nil && !errors.Is(err, services.ErrTeamMemberAlreadyExists) {
			if createdUser {
				_ = h.users.Delete(ctx, user.ID)
			}
			switch {
			case errors.Is(err, services.ErrTeamNotFound):
				response.Error(c, appErrors.NewBadRequest("Assigned team no longer exists"))
			default:
				response.Error(c, err)
			}
			return
		}
		if err == nil {
			addedToTeam = true
		}
	}

	if err := h.invites.AcceptInvite(ctx, invite.ID); err != nil {
		if invite.TeamID != nil && h.teams != nil && addedToTeam {
			_ = h.teams.RemoveMember(ctx, *invite.TeamID, user.ID)
		}
		if createdUser {
			_ = h.users.Delete(ctx, user.ID)
		}
		switch {
		case errors.Is(err, services.ErrInviteAlreadyUsed):
			response.Error(c, appErrors.NewBadRequest("Invite has already been used"))
		case errors.Is(err, services.ErrInviteExpired):
			response.Error(c, appErrors.NewBadRequest("Invite token has expired"))
		default:
			response.Error(c, appErrors.ErrInternalServer)
		}
		return
	}

	message := "Account created successfully. You can now sign in."
	if !createdUser {
		message = "Team access granted successfully. You can now sign in."
	}
	if requiresVerification {
		message = "Account created. Please check your email to verify and activate your account."
	}

	payload := redeemInviteResponse{
		User: inviteUserDTO{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			IsActive:  user.IsActive,
		},
		Message: message,
	}

	response.Success(c, http.StatusCreated, payload)
}

func toInviteDTO(invite *models.UserInvite, now time.Time) inviteDTO {
	status := "pending"
	switch {
	case invite == nil:
		status = "pending"
	case invite.AcceptedAt != nil:
		status = "accepted"
	case invite.ExpiresAt.Before(now):
		status = "expired"
	default:
		status = "pending"
	}

	var teamID string
	if invite != nil && invite.TeamID != nil {
		teamID = *invite.TeamID
	}

	var teamName string
	if invite != nil && invite.Team != nil {
		teamName = invite.Team.Name
	}

	return inviteDTO{
		ID:         invite.ID,
		Email:      invite.Email,
		InvitedBy:  invite.InvitedBy,
		CreatedAt:  invite.CreatedAt,
		ExpiresAt:  invite.ExpiresAt,
		AcceptedAt: invite.AcceptedAt,
		Status:     status,
		TeamID:     teamID,
		TeamName:   teamName,
	}
}
