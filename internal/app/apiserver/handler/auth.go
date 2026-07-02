package handler

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/apierr"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

var (
	ErrUnauthenticated   = errorx.New(errorx.KindUnauthenticated, "auth: not authenticated")
	ErrServiceDegraded   = errorx.New(errorx.KindServiceDegraded, "auth: upstream failure")
	ErrUserNotFoundLocal = errorx.New(errorx.KindNotFound, "auth: user not found")
)

// AuthMe returns the authenticated user. Errors are returned as
// *api.Error so ogen picks them up via the AuthMeRes interface;
// the status code and wire body come from apierr.FromError so the
// same errorx.Kind produces the same response as the WithErrorHandler.
func (h *Handler) AuthMe(ctx context.Context) (api.AuthMeRes, error) {
	uid := h.userID(ctx)
	if uid == "" {
		return apiErr(errorx.New(errorx.KindUnauthenticated, "auth: not authenticated")), nil
	}
	u, err := h.users.GetByID(ctx, uid)
	if err != nil {
		return apiErr(errorx.New(errorx.KindServiceDegraded, fmt.Sprintf("auth: user lookup failed (uid=%s, err=%v)", uid, err))), nil
	}
	if u == nil {
		return apiErr(errorx.New(errorx.KindNotFound, "auth: user not found")), nil
	}
	return userToAPI(u), nil
}

// apiErr converts an errorx error to the api.Error wire format.
// Returned as a pointer so the AuthMeRes interface dispatches to
// the error branch (not the *User branch) and ogen emits the
// correct status code.
func apiErr(err error) *api.Error {
	_, e := apierr.FromError(err)
	return &api.Error{
		Code:    api.ErrorCode(e.Code),
		Message: e.Message,
	}
}
