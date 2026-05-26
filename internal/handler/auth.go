package handler

import (
	"context"
	"net/url"

	api "github.com/mandacode-labs/retrowin-go/pkg/api"

	"github.com/mandacode-labs/retrowin-go/internal/auth"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
	"github.com/mandacode-labs/retrowin-go/internal/middleware"
)

// InitiateLogin implements GET /auth/login.
func (h *Handler) InitiateLogin(ctx context.Context) (api.InitiateLoginRes, error) {
	resp, err := h.authSvc.InitiateLogin(ctx)
	if err != nil {
		return nil, err
	}

	authURL, err := url.Parse(resp.AuthorizationURL)
	if err != nil {
		return nil, err
	}

	return &api.LoginResponse{
		AuthorizationUrl: *authURL,
		State:            resp.State,
	}, nil
}

// HandleCallback implements GET /auth/callback.
func (h *Handler) HandleCallback(ctx context.Context, params api.HandleCallbackParams) (api.HandleCallbackRes, error) {
	// Validate required parameters
	if params.Code == "" {
		return &api.HandleCallbackBadRequest{
			Error: api.ErrorError{
				Type:    "invalid_request",
				Message: "code parameter is required",
			},
		}, nil
	}
	if params.State == "" {
		return &api.HandleCallbackBadRequest{
			Error: api.ErrorError{
				Type:    "invalid_request",
				Message: "state parameter is required",
			},
		}, nil
	}

	callbackReq := &auth.CallbackRequest{
		Code:  params.Code,
		State: params.State,
	}

	resp, err := h.authSvc.HandleCallback(ctx, callbackReq)
	if err != nil {
		if errors.IsUnauthorized(err) || errors.IsNotFound(err) {
			return &api.HandleCallbackUnauthorized{
				Error: api.ErrorError{
					Type:    "authentication_error",
					Message: err.Error(),
				},
			}, nil
		}
		return &api.HandleCallbackBadRequest{
			Error: api.ErrorError{
				Type:    "invalid_request",
				Message: err.Error(),
			},
		}, nil
	}

	return &api.CallbackResponse{
		SessionId: resp.SessionID,
		UserId:    resp.UserID,
		ExpiresAt: api.OptTimestamp{Value: api.Timestamp(resp.ExpiresAt), Set: true},
	}, nil
}

// Logout implements POST /auth/logout.
func (h *Handler) Logout(ctx context.Context) (*api.LogoutResponse, error) {
	sessionID := middleware.GetSessionID(ctx)
	if sessionID == "" {
		return &api.LogoutResponse{LogoutUrl: ""}, nil
	}

	resp, err := h.authSvc.Logout(ctx, sessionID)
	if err != nil {
		return &api.LogoutResponse{LogoutUrl: ""}, nil
	}

	return &api.LogoutResponse{LogoutUrl: resp.LogoutURL}, nil
}
