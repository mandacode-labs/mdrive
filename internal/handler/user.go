package handler

import (
	"context"

	api "github.com/mandacode-labs/retrowin-go/pkg/api"

	"github.com/mandacode-labs/retrowin-go/internal/errors"
	"github.com/mandacode-labs/retrowin-go/internal/middleware"
	extuser "github.com/mandacode-labs/retrowin-go/internal/user"
)

// GetUser implements GET /user.
func (h *Handler) GetUser(ctx context.Context) (api.GetUserRes, error) {
	userID := middleware.GetUserID(ctx)
	if userID == "" {
		return &api.GetUserUnauthorized{
			Error: api.ErrorError{
				Type:    "authentication_error",
				Message: "no user ID in context",
			},
		}, nil
	}

	u, err := h.extUserSvc.GetByID(ctx, userID)
	if err != nil {
		if errors.IsNotFound(err) {
			return &api.GetUserNotFound{
				Error: api.ErrorError{
					Type:    "not_found",
					Message: "user not found",
				},
			}, nil
		}
		return &api.GetUserUnauthorized{
			Error: api.ErrorError{
				Type:    "authentication_error",
				Message: err.Error(),
			},
		}, nil
	}

	return &api.UserResponse{
		User: *h.toExtUser(u),
	}, nil
}

// DeleteUser implements DELETE /user.
func (h *Handler) DeleteUser(ctx context.Context) (api.DeleteUserRes, error) {
	userID := middleware.GetUserID(ctx)
	if userID == "" {
		return &api.DeleteUserUnauthorized{
			Error: api.ErrorError{
				Type:    "authentication_error",
				Message: "no user ID in context",
			},
		}, nil
	}

	// Get user first to obtain provider info for deletion
	u, err := h.extUserSvc.GetByID(ctx, userID)
	if err != nil {
		if errors.IsNotFound(err) {
			return &api.DeleteUserNotFound{
				Error: api.ErrorError{
					Type:    "not_found",
					Message: "user not found",
				},
			}, nil
		}
		return &api.DeleteUserUnauthorized{
			Error: api.ErrorError{
				Type:    "authentication_error",
				Message: err.Error(),
			},
		}, nil
	}

	err = h.extUserSvc.Delete(ctx, u.Provider(), u.ProviderID())
	if err != nil {
		if errors.IsNotFound(err) {
			return &api.DeleteUserNotFound{
				Error: api.ErrorError{
					Type:    "not_found",
					Message: "user not found",
				},
			}, nil
		}
		return &api.DeleteUserUnauthorized{
			Error: api.ErrorError{
				Type:    "authentication_error",
				Message: err.Error(),
			},
		}, nil
	}

	return &api.DeleteUserNoContent{}, nil
}

func (h *Handler) toExtUser(u *extuser.User) *api.User {
	return &api.User{
		ID:         u.ID(),
		Provider:   api.Provider(u.Provider()),
		ProviderId: u.ProviderID(),
		JoinDate:   toOptTimestamp(u.JoinDate()),
		CreatedAt:  toOptTimestamp(u.CreatedAt()),
		UpdatedAt:  toOptTimestamp(u.UpdatedAt()),
	}
}
