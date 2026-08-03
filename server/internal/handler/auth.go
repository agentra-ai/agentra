package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/agentra-ai/agentra/server/internal/auth"
	"github.com/agentra-ai/agentra/server/internal/logger"
	"github.com/agentra-ai/agentra/server/internal/otpcode"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	errSignupDisabled            = errors.New("signup is disabled; ask an administrator for an invitation")
	errSignupNotAllowed          = errors.New("this email address is not allowed to sign up")
	errWorkspaceCreationDisabled = errors.New("workspace creation is disabled; ask an administrator for an invitation")
)

type UserResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func userToResponse(u db.User) UserResponse {
	return UserResponse{
		ID:        uuidToString(u.ID),
		Name:      u.Name,
		Email:     u.Email,
		AvatarURL: textToPtr(u.AvatarUrl),
		CreatedAt: timestampToString(u.CreatedAt),
		UpdatedAt: timestampToString(u.UpdatedAt),
	}
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type SendCodeRequest struct {
	Email string `json:"email"`
}

type SendCodeResponse struct {
	Message string  `json:"message"`
	DevCode *string `json:"dev_code,omitempty"`
}

type VerifyCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func defaultWorkspaceName(user db.User) string {
	name := strings.TrimSpace(user.Name)
	if name == "" {
		email := strings.TrimSpace(user.Email)
		if at := strings.Index(email, "@"); at > 0 {
			name = email[:at]
		}
	}
	if name == "" {
		name = "Personal"
	}
	return name + "'s Workspace"
}

func slugifyWorkspacePart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastWasDash := false

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasDash = false
		case b.Len() > 0 && !lastWasDash:
			b.WriteByte('-')
			lastWasDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}

func defaultWorkspaceSlug(user db.User) string {
	candidates := []string{
		slugifyWorkspacePart(user.Name),
		slugifyWorkspacePart(strings.Split(strings.TrimSpace(user.Email), "@")[0]),
		"workspace",
	}

	base := "workspace"
	for _, candidate := range candidates {
		if candidate != "" {
			base = candidate
			break
		}
	}

	userID := uuidToString(user.ID)
	if len(userID) >= 8 {
		return base + "-" + userID[:8]
	}
	return base
}

func (h *Handler) ensureUserWorkspace(ctx context.Context, user db.User) error {
	workspaces, err := h.Queries.ListWorkspaces(ctx, user.ID)
	if err != nil {
		return err
	}
	if len(workspaces) > 0 {
		return nil
	}
	if h.AccessPolicy.WorkspaceCreationDisabled {
		return errWorkspaceCreationDisabled
	}

	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := h.Queries.WithTx(tx)
	workspaces, err = qtx.ListWorkspaces(ctx, user.ID)
	if err != nil {
		return err
	}
	if len(workspaces) > 0 {
		return nil
	}
	if h.AccessPolicy.WorkspaceCreationDisabled {
		return errWorkspaceCreationDisabled
	}

	wsName := defaultWorkspaceName(user)
	workspace, err := qtx.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		Name:        wsName,
		Slug:        defaultWorkspaceSlug(user),
		Description: pgtype.Text{},
		IssuePrefix: generateIssuePrefix(wsName),
	})
	if err != nil {
		if isUniqueViolation(err) {
			workspaces, lookupErr := h.Queries.ListWorkspaces(ctx, user.ID)
			if lookupErr == nil && len(workspaces) > 0 {
				return nil
			}
		}
		return err
	}

	if _, err := qtx.CreateMember(ctx, db.CreateMemberParams{
		WorkspaceID: workspace.ID,
		UserID:      user.ID,
		Role:        "owner",
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// emailDomain extracts the domain part of an email address (e.g. "alice@acme.com" -> "acme.com").
func emailDomain(email string) string {
	idx := strings.LastIndex(email, "@")
	if idx < 0 || idx == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[idx+1:])
}

func validLoginEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && emailDomain(email) != ""
}

func (h *Handler) checkSignupAccess(ctx context.Context, email string) error {
	_, err := h.Queries.GetUserByEmail(ctx, email)
	if err == nil {
		return nil
	}
	if !isNotFound(err) {
		return err
	}

	if h.AccessPolicy.SignupDisabled {
		return errSignupDisabled
	}
	if !h.AccessPolicy.AllowsNewSignup(email) {
		return errSignupNotAllowed
	}
	if !h.AccessPolicy.WorkspaceCreationDisabled {
		return nil
	}

	domain := emailDomain(email)
	workspace, err := h.Queries.FindWorkspaceByEmailDomain(ctx, domain)
	if err != nil {
		if isNotFound(err) {
			return errWorkspaceCreationDisabled
		}
		return err
	}
	if !workspace.ID.Valid {
		return errWorkspaceCreationDisabled
	}
	return nil
}

func isAccessPolicyError(err error) bool {
	return errors.Is(err, errSignupDisabled) ||
		errors.Is(err, errSignupNotAllowed) ||
		errors.Is(err, errWorkspaceCreationDisabled)
}

func generateCode() (string, error) {
	return otpcode.Generate()
}

func (h *Handler) issueJWT(user db.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   uuidToString(user.ID),
		"email": user.Email,
		"name":  user.Name,
		"exp":   time.Now().Add(72 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	return token.SignedString(auth.JWTSecret())
}

func (h *Handler) findOrCreateUser(ctx context.Context, email string) (db.User, error) {
	user, err := h.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		if !isNotFound(err) {
			return db.User{}, err
		}
		name := email
		if at := strings.Index(email, "@"); at > 0 {
			name = email[:at]
		}
		user, err = h.Queries.CreateUser(ctx, db.CreateUserParams{
			Name:  name,
			Email: email,
		})
		if err != nil {
			return db.User{}, err
		}
	}
	return user, nil
}

func (h *Handler) SendCode(w http.ResponseWriter, r *http.Request) {
	var req SendCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !validLoginEmail(email) {
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return
	}

	if err := h.checkSignupAccess(r.Context(), email); err != nil {
		if isAccessPolicyError(err) {
			writeError(w, http.StatusForbidden, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "failed to evaluate signup policy")
		}
		return
	}

	// Rate limit: max 1 code per 10 seconds per email
	latest, err := h.Queries.GetLatestCodeByEmail(r.Context(), email)
	if err == nil && time.Since(latest.CreatedAt.Time) < 10*time.Second {
		writeError(w, http.StatusTooManyRequests, "please wait before requesting another code")
		return
	}

	code, err := generateCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate code")
		return
	}

	verification, err := h.Queries.CreateVerificationCode(r.Context(), db.CreateVerificationCodeParams{
		Email:     email,
		Code:      code,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store verification code")
		return
	}

	devCode, err := h.EmailService.SendVerificationCode(r.Context(), email, code)
	if err != nil {
		_ = h.Queries.MarkVerificationCodeUsed(r.Context(), verification.ID)
		writeError(w, http.StatusInternalServerError, "failed to send verification code")
		return
	}

	// Best-effort cleanup of expired codes
	_ = h.Queries.DeleteExpiredVerificationCodes(r.Context())

	writeJSON(w, http.StatusOK, SendCodeResponse{
		Message: "Verification code sent",
		DevCode: devCode,
	})
}

func (h *Handler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	var req VerifyCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	code := strings.TrimSpace(req.Code)

	if email == "" || code == "" {
		writeError(w, http.StatusBadRequest, "email and code are required")
		return
	}
	if !validLoginEmail(email) {
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return
	}
	if err := h.checkSignupAccess(r.Context(), email); err != nil {
		if isAccessPolicyError(err) {
			writeError(w, http.StatusForbidden, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "failed to evaluate signup policy")
		}
		return
	}

	dbCode, err := h.Queries.GetLatestVerificationCode(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	if subtle.ConstantTimeCompare([]byte(code), []byte(dbCode.Code)) != 1 {
		_ = h.Queries.IncrementVerificationCodeAttempts(r.Context(), dbCode.ID)
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	if err := h.Queries.MarkVerificationCodeUsed(r.Context(), dbCode.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify code")
		return
	}

	user, err := h.findOrCreateUser(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// JIT provisioning: if the user's email domain matches a claimed workspace,
	// auto-add them as a member. Best-effort — never blocks login.
	ctx := r.Context()
	if domain := emailDomain(email); domain != "" {
		if ws, qerr := h.Queries.FindWorkspaceByEmailDomain(ctx, domain); qerr == nil && ws.ID.Valid {
			if _, merr := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{
				UserID:      user.ID,
				WorkspaceID: ws.ID,
			}); merr != nil {
				// Not yet a member — auto-add.
				if _, cerr := h.Queries.CreateMember(ctx, db.CreateMemberParams{
					WorkspaceID: ws.ID,
					UserID:      user.ID,
					Role:        "member",
				}); cerr == nil {
					slog.Info("SSO: auto-provisioned user to workspace",
						"user_id", util.UUIDToString(user.ID), "workspace_id", util.UUIDToString(ws.ID), "domain", domain)
				}
			}
		}
	}

	if err := h.ensureUserWorkspace(r.Context(), user); err != nil {
		if errors.Is(err, errWorkspaceCreationDisabled) {
			writeError(w, http.StatusForbidden, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "failed to provision workspace")
		}
		return
	}

	tokenString, err := h.issueJWT(user)
	if err != nil {
		slog.Warn("login failed", append(logger.RequestAttrs(r), "error", err, "email", req.Email)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Set CloudFront signed cookies for CDN access.
	if h.CFSigner != nil {
		for _, cookie := range h.CFSigner.SignedCookies(time.Now().Add(72 * time.Hour)) {
			http.SetCookie(w, cookie)
		}
	}

	slog.Info("user logged in", append(logger.RequestAttrs(r), "user_id", uuidToString(user.ID), "email", user.Email)...)
	writeJSON(w, http.StatusOK, LoginResponse{
		Token: tokenString,
		User:  userToResponse(user),
	})
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(user))
}

type UpdateMeRequest struct {
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req UpdateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	currentUser, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	name := currentUser.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
	}

	params := db.UpdateUserParams{
		ID:   currentUser.ID,
		Name: name,
	}
	if req.AvatarURL != nil {
		params.AvatarUrl = pgtype.Text{String: strings.TrimSpace(*req.AvatarURL), Valid: true}
	}

	updatedUser, err := h.Queries.UpdateUser(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(updatedUser))
}
