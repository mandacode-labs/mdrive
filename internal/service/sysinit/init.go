package sysinit

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/application/vfs"
	"github.com/mandacode-labs/mdrive/internal/core/inode"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errors"
	"github.com/mandacode-labs/mdrive/internal/system"
)

type InitService interface {
	InitSystem(ctx context.Context, cmd *InitSystemCommand) (*InitResult, error)
}

type InitSystemCommand struct {
	Name         string
	Description  *string
	RootUserID   string        // External user ID for root
	InitialUsers []InitialUser // Optional initial users to create
}

type InitialUser struct {
	UserID   string
	Username string
}

type InitResult struct {
	System       *system.System
	RootUser     *user.SystemUser
	InitialUsers []*user.SystemUser
	RootDir      *inode.Inode
	HomeDir      *inode.Inode
	TrashDir     *inode.Inode
}

type service struct {
	systemSvc system.SystemService
	userSvc   UserService
	fsSvc     vfs.VFS
	inodeOps  InodeService
}

func NewService(systemSvc system.SystemService, userSvc UserService, fsSvc vfs.VFS, inodeOps InodeService) InitService {
	return &service{
		systemSvc: systemSvc,
		userSvc:   userSvc,
		fsSvc:     fsSvc,
		inodeOps:  inodeOps,
	}
}

func (s *service) InitSystem(ctx context.Context, cmd *InitSystemCommand) (*InitResult, error) {
	if cmd.Name == "" {
		return nil, errors.BadRequest("name is required")
	}

	sys, err := s.systemSvc.Create(ctx, &system.CreateCommand{
		Name:        cmd.Name,
		Description: cmd.Description,
		Status:      system.StatusActive,
	})
	if err != nil {
		return nil, err
	}

	rootUser, err := s.createRootUser(ctx, sys.ID(), cmd.RootUserID)
	if err != nil {
		return nil, err
	}

	initialUsers := make([]*user.SystemUser, 0, len(cmd.InitialUsers))
	for _, u := range cmd.InitialUsers {
		sysUser, err := s.userSvc.Create(ctx, &user.CreateCommand{
			UserID:   u.UserID,
			SystemID: sys.ID(),
			Username: u.Username,
			UID:      -1, // Auto-assign UID
		})
		if err != nil {
			return nil, err
		}
		initialUsers = append(initialUsers, sysUser)
	}

	rootDir, err := s.createRootDirectory(ctx, sys.ID())
	if err != nil {
		return nil, err
	}

	homeDir, err := s.createHomeDirectory(ctx, sys.ID(), rootDir.ID())
	if err != nil {
		return nil, err
	}

	trashDir, err := s.createTrashDirectory(ctx, sys.ID(), homeDir.ID())
	if err != nil {
		return nil, err
	}

	return &InitResult{
		System:       sys,
		RootUser:     rootUser,
		InitialUsers: initialUsers,
		RootDir:      rootDir,
		HomeDir:      homeDir,
		TrashDir:     trashDir,
	}, nil
}

func (s *service) createRootUser(ctx context.Context, systemID, rootUserID string) (*user.SystemUser, error) {
	return s.userSvc.Create(ctx, &user.CreateCommand{
		UserID:   rootUserID,
		SystemID: systemID,
		Username: "root",
		UID:      0, // Root UID
	})
}

func (s *service) createRootDirectory(ctx context.Context, systemID string) (*inode.Inode, error) {
	rootDir, err := s.fsSvc.CreateDirectory(ctx, &vfs.CreateDirectoryCommand{
		SystemID: systemID,
		UID:      0, // root
		GID:      0, // root group
		Mode:     inode.ModeDirectory | inode.PermOwnerRWX | inode.PermGroupRX | inode.PermOtherRX,
		Flags:    inode.FlagRoot, // Mark as root directory
	})
	if err != nil {
		return nil, err
	}
	return rootDir, nil
}

func (s *service) createHomeDirectory(ctx context.Context, systemID string, rootDirID string) (*inode.Inode, error) {
	homeDir, err := s.fsSvc.CreateDirectory(ctx, &vfs.CreateDirectoryCommand{
		SystemID: systemID,
		UID:      0, // root
		GID:      0, // root group
		Mode:     inode.ModeDirectory | inode.PermOwnerRWX | inode.PermGroupRX | inode.PermOtherRX,
	})
	if err != nil {
		return nil, err
	}

	// Link home to root directory
	if err := s.inodeOps.Link(ctx, rootDirID, inode.DirEntry{
		Name:     "home",
		InodeID:  homeDir.ID(),
		FileType: uint8(inode.ModeDirectory >> 12),
	}); err != nil {
		return nil, err
	}

	// Add ".." entry to home directory
	if err := s.inodeOps.Link(ctx, homeDir.ID(), inode.DirEntry{
		Name:     "..",
		InodeID:  rootDirID,
		FileType: uint8(inode.ModeDirectory >> 12),
	}); err != nil {
		return nil, err
	}

	return homeDir, nil
}

func (s *service) createTrashDirectory(ctx context.Context, systemID string, homeDirID string) (*inode.Inode, error) {
	trashDir, err := s.fsSvc.CreateDirectory(ctx, &vfs.CreateDirectoryCommand{
		SystemID: systemID,
		UID:      0, // root
		GID:      0, // root group
		Mode:     inode.ModeDirectory | inode.PermOwnerRWX | inode.PermGroupRX | inode.PermOtherRX,
	})
	if err != nil {
		return nil, err
	}

	// Link .trash to home directory
	if err := s.inodeOps.Link(ctx, homeDirID, inode.DirEntry{
		Name:     ".trash",
		InodeID:  trashDir.ID(),
		FileType: uint8(inode.ModeDirectory >> 12),
	}); err != nil {
		return nil, err
	}

	// Add ".." entry to trash directory
	if err := s.inodeOps.Link(ctx, trashDir.ID(), inode.DirEntry{
		Name:     "..",
		InodeID:  homeDirID,
		FileType: uint8(inode.ModeDirectory >> 12),
	}); err != nil {
		return nil, err
	}

	return trashDir, nil
}
