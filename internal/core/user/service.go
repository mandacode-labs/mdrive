package user

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// Service provides domain-level user operations.
type Service struct {
	repo Repository
}

// NewService creates a new Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// UpsertFromOIDC creates or updates a user from OIDC claims.
func (s *Service) UpsertFromOIDC(ctx context.Context, cmd *CreateCommand) (*User, error) {
	logx.Debug(ctx, "user.service.upsert.enter",
		slog.String("provider", cmd.Provider),
		slog.String("provider_id", cmd.ProviderID),
	)
	if cmd.Provider == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "user: provider is required")
	}
	if cmd.ProviderID == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "user: provider_id is required")
	}
	if cmd.Name == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "user: name is required")
	}

	existing, err := s.repo.GetByProviderID(ctx, cmd.Provider, cmd.ProviderID)
	if err != nil {
		logx.Debug(ctx, "user.service.upsert.lookup_err",
			slog.String("err", err.Error()),
		)
		return nil, errorx.Wrap(err, "user: upsert lookup", errorx.KindUnavailable)
	}
	if existing != nil {
		logx.Debug(ctx, "user.service.upsert.existing",
			slog.String("user_id", existing.ID()),
			slog.Bool("needs_update", existing.Name() != cmd.Name || emailDiffers(existing, cmd)),
		)
		if existing.Name() != cmd.Name || emailDiffers(existing, cmd) {
			updated := NewUser(
				existing.ID(), existing.PublicID(), cmd.Name, cmd.Email,
				existing.Provider(), existing.ProviderID(),
				existing.CreatedAt(), time.Now(),
			)
			saved, err := s.repo.Update(ctx, updated)
			if err != nil {
				logx.Debug(ctx, "user.service.upsert.update_err",
					slog.String("err", err.Error()),
				)
				return nil, errorx.Wrap(err, "user: upsert update", errorx.KindUnavailable)
			}
			logx.Debug(ctx, "user.service.upsert.updated",
				slog.String("user_id", saved.ID()),
			)
			return saved, nil
		}
		logx.Debug(ctx, "user.service.upsert.unchanged",
			slog.String("user_id", existing.ID()),
		)
		return existing, nil
	}

	created, err := s.repo.Create(ctx, cmd)
	if err != nil {
		logx.Debug(ctx, "user.service.upsert.create_err",
			slog.String("err", err.Error()),
		)
		return nil, errorx.Wrap(err, "user: upsert create", errorx.KindUnavailable)
	}
	logx.Debug(ctx, "user.service.upsert.created",
		slog.String("user_id", created.ID()),
	)
	return created, nil
}

// GetByID returns a user by private ID.
func (s *Service) GetByID(ctx context.Context, id string) (*User, error) {
	logx.Debug(ctx, "user.service.get_by_id.enter", slog.String("user_id", id))
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		logx.Debug(ctx, "user.service.get_by_id.err", slog.String("err", err.Error()))
		return nil, errorx.Wrap(err, fmt.Sprintf("user: get by id (id=%s)", id))
	}
	if u == nil {
		logx.Debug(ctx, "user.service.get_by_id.not_found", slog.String("user_id", id))
		return nil, errorx.New(errorx.KindNotFound, "user: not found (id="+id+")")
	}
	logx.Debug(ctx, "user.service.get_by_id.ok", slog.String("user_id", u.ID()))
	return u, nil
}

// GetByPublicID returns a user by public ID.
func (s *Service) GetByPublicID(ctx context.Context, publicID string) (*User, error) {
	logx.Debug(ctx, "user.service.get_by_public_id.enter", slog.String("public_id", publicID))
	u, err := s.repo.GetByPublicID(ctx, publicID)
	if err != nil {
		logx.Debug(ctx, "user.service.get_by_public_id.err", slog.String("err", err.Error()))
		return nil, errorx.Wrap(err, fmt.Sprintf("user: get by public id (public_id=%s)", publicID))
	}
	if u == nil {
		logx.Debug(ctx, "user.service.get_by_public_id.not_found", slog.String("public_id", publicID))
		return nil, errorx.New(errorx.KindNotFound, "user: not found (public_id="+publicID+")")
	}
	logx.Debug(ctx, "user.service.get_by_public_id.ok", slog.String("user_id", u.ID()))
	return u, nil
}

// GetByProviderID returns a user by (provider, providerID).
func (s *Service) GetByProviderID(ctx context.Context, provider, providerID string) (*User, error) {
	logx.Debug(ctx, "user.service.get_by_provider_id.enter",
		slog.String("provider", provider),
		slog.String("provider_id", providerID),
	)
	u, err := s.repo.GetByProviderID(ctx, provider, providerID)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("user: get by provider id (provider=%s, provider_id=%s)", provider, providerID))
	}
	return u, nil
}

// Update updates an existing user.
func (s *Service) Update(ctx context.Context, u *User) (*User, error) {
	if u == nil {
		return nil, errorx.New(errorx.KindInternal, "user: update requires non-nil user")
	}
	saved, err := s.repo.Update(ctx, u)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("user: update (id=%s)", u.ID()))
	}
	return saved, nil
}

func emailDiffers(u *User, cmd *CreateCommand) bool {
	if cmd.Email == nil && u.Email() == nil {
		return false
	}
	if cmd.Email == nil || u.Email() == nil {
		return true
	}
	return *u.Email() != *cmd.Email
}
