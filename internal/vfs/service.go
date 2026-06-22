package vfs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/crypto"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
)

// ContentCipher returns the NodeCipher for the given driveID, or
// (nil, nil) for drives that predate envelope encryption. The vfs
// service uses this to encrypt/decrypt node content at the
// storage boundary. Returning (nil, nil) means "store as plaintext".
// The ctx is the request context so the lookup can hit the database
// (or, in future revisions, a cache).
type ContentCipher func(ctx context.Context, driveID string) (*crypto.NodeCipher, error)

// Reencryptor is an optional dependency for migrating plaintext
// objects (e.g. those uploaded via a presigned URL) to ciphertext.
// CompleteUpload calls MigratePlaintext after the client finishes
// the upload so the body sitting in S3 is the encrypted form.
// When nil, presigned-uploaded bodies stay plaintext; this matches
// the pre-Phase-3b behaviour and is fine for dev or for drives that
// predate envelope encryption.
type Reencryptor interface {
	MigratePlaintext(ctx context.Context, driveID, bucket, srcKey, dstKey string) error
}

// Compile-time interface satisfaction: core services satisfy vfs-declared interfaces.
var (
	_ NodeClient  = (*node.Service)(nil)
	_ DriveClient = (*drive.Service)(nil)
	_ UserClient  = (*user.Service)(nil)
	_ PermClient  = (*permission.OpenFGAChecker)(nil)
)

// --------------- Consumer-declared interfaces ---------------

// NodeClient is the consumer-declared interface for node-domain operations.
// The consumer (vfs) declares it so concrete implementations satisfy it implicitly.
type NodeClient interface {
	CreateFile(ctx context.Context, content string) (*node.Node, error)
	CreateDirectory(ctx context.Context) (*node.Node, error)
	CreateSymlink(ctx context.Context, target string) (*node.Node, error)
	CreateObject(ctx context.Context, content node.ObjectContent, size int64) (*node.Node, error)
	CreateMount(ctx context.Context, sourceDriveID string) (*node.Node, error)
	Link(ctx context.Context, parent *node.Node, name string, child *node.Node) error
	BulkLink(ctx context.Context, parent *node.Node, entries map[string]*node.Node) error
	Unlink(ctx context.Context, parent *node.Node, name string) (*node.Node, error)
	BulkUnlink(ctx context.Context, parent *node.Node, names []string) ([]*node.Node, error)
	UnlinkOrReplace(ctx context.Context, parent *node.Node, name string) (*node.Node, error)
	GetByID(ctx context.Context, id uuid.UUID) (*node.Node, error)
	Save(ctx context.Context, n *node.Node) error
	Delete(ctx context.Context, id uuid.UUID) error
	WithTx(ctx context.Context, fn func(tx *node.Service) error) error
}

// DriveClient is the consumer-declared interface for drive-domain operations.
type DriveClient interface {
	Create(ctx context.Context, name string, desc *string, ownerID string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error)
	GetByID(ctx context.Context, id string) (*drive.Drive, error)
	GetByPublicID(ctx context.Context, pubID string) (*drive.Drive, error)
	GetStorage(ctx context.Context, driveID string) (*drive.Storage, error)
	Update(ctx context.Context, id string, name, description *string) (*drive.Drive, error)
	Delete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (*drive.Drive, error)
	ListDeleted(ctx context.Context, before time.Time, limit int) ([]*drive.Drive, error)
	ListByOwner(ctx context.Context, ownerID string) ([]*drive.Drive, error)
}

// UserClient is the consumer-declared interface for user-domain
// operations the VFS actually uses (upsert on OIDC login, lookup by
// private id). Lookups by public id, provider id, and updates are
// not part of the VFS surface; callers that need them should use the
// user.Service directly.
type UserClient interface {
	UpsertFromOIDC(ctx context.Context, cmd *user.CreateCommand) (*user.User, error)
	GetByID(ctx context.Context, id string) (*user.User, error)
}

// PermClient is the consumer-declared interface for permission checks.
type PermClient interface {
	Check(ctx context.Context, userID, relation, objType, objID string) (bool, error)
	Grant(ctx context.Context, userID, relation, objType, objID string) error
}

// TombstoneInserter records S3 object references for deferred deletion by the GC worker.
type TombstoneInserter interface {
	InsertTombstones(ctx context.Context, refs []ObjectRef) error
}

// ObjectRef is a reference to an S3 object that needs cleanup.
type ObjectRef struct {
	Bucket string
	Key    string
}

// --------------- VFS Service ---------------

// Service is the VFS orchestration layer.
type Service struct {
	Node          NodeClient
	Drive         DriveClient
	User          UserClient
	Store         Store
	Perm          PermClient
	Reg           upload.Registry
	GC            TombstoneInserter
	path          *resolver
	contentCipher ContentCipher
	Reencryptor   Reencryptor
}

// NewService creates a new VFS Service. contentCipher and reencryptor
// are optional; pass nil for either to skip envelope encryption
// (the repository then stores node content as plaintext, which is
// fine for dev/test and for drives that predate Phase 3a/3b).
func NewService(
	n NodeClient,
	d DriveClient,
	u UserClient,
	store Store,
	checker PermClient,
	reg upload.Registry,
	gc TombstoneInserter,
	contentCipher ContentCipher,
	reencryptor Reencryptor,
) *Service {
	if reg == nil {
		reg = upload.NewMemoryRegistry()
	}
	return &Service{
		Node:          n,
		Drive:         d,
		User:          u,
		Store:         store,
		Perm:          checker,
		Reg:           reg,
		GC:            gc,
		path:          newResolver(n),
		contentCipher: contentCipher,
		Reencryptor:   reencryptor,
	}
}

// WithNodeTx executes fn within a node-domain transaction.
// Only Node operations are transactional; Store, Reg, Perm, and GC are NOT rolled back.
func (s *Service) WithNodeTx(ctx context.Context, fn func(tx *Service) error) error {
	return s.Node.WithTx(ctx, func(txNode *node.Service) error {
		return fn(&Service{
			Node:          txNode,
			Drive:         s.Drive,
			User:          s.User,
			Store:         s.Store,
			Perm:          s.Perm,
			Reg:           s.Reg,
			GC:            s.GC,
			path:          newResolver(txNode),
			contentCipher: s.contentCipher,
			Reencryptor:   s.Reencryptor,
		})
	})
}

// Resolved is the result of a path resolution across drives.
type Resolved struct {
	// DriveID is the drive the resolved node lives in. If the path
	// crossed mount nodes, this is the source drive; otherwise it
	// equals the driveID passed to Resolve.
	DriveID string
	Node    *node.Node
}

// nodeCipherFor returns the NodeCipher for driveID, or (nil, nil) when
// envelope encryption is disabled or the drive has no wrapped DEK
// (e.g. predates Phase 3a). The two no-cipher cases are deliberately
// indistinguishable to callers: both mean "store as plaintext".
func (s *Service) nodeCipherFor(ctx context.Context, driveID string) (*crypto.NodeCipher, error) {
	if s.contentCipher == nil {
		return nil, nil
	}
	return s.contentCipher(ctx, driveID)
}

// encryptContent encrypts n's raw content in place using the drive's
// NodeCipher and AAD-binds to (driveID, n.ID). No-op when envelope
// encryption is disabled or the drive has no DEK. Callers must run
// this before any Repository write that persists the child row.
func (s *Service) encryptContent(ctx context.Context, driveID string, n *node.Node) error {
	nc, err := s.nodeCipherFor(ctx, driveID)
	if err != nil {
		return fmt.Errorf("vfs: encrypt: %w", err)
	}
	if nc == nil {
		return nil
	}
	pt := []byte(n.Content())
	if len(pt) == 0 {
		return nil
	}
	ct, err := nc.Encrypt(pt, driveID, n.ID())
	if err != nil {
		return fmt.Errorf("vfs: encrypt: %w", err)
	}
	n.SetContent(ct)
	return nil
}

// decryptContent decrypts n's raw content in place. No-op when
// envelope encryption is disabled or the drive has no DEK. Callers
// must run this after Repository read but before any code that
// interprets n's content (ReadFile, ReadSymlink, ReadObject).
func (s *Service) decryptContent(ctx context.Context, driveID string, n *node.Node) error {
	nc, err := s.nodeCipherFor(ctx, driveID)
	if err != nil {
		return fmt.Errorf("vfs: decrypt: %w", err)
	}
	if nc == nil {
		return nil
	}
	ct := []byte(n.Content())
	if len(ct) == 0 {
		return nil
	}
	pt, err := nc.Decrypt(ct, driveID, n.ID())
	if err != nil {
		return fmt.Errorf("vfs: decrypt: %w", err)
	}
	n.SetContent(pt)
	return nil
}

// maxMountHops caps the mount-traversal depth so a malicious or
// pathological mount graph cannot spin a single resolve forever.
const maxMountHops = 32

// Resolve walks a path starting at driveID, following mount nodes
// when encountered. Returns the drive id the final node lives in
// (which may differ from driveID if mounts were crossed) and the
// node itself.
func (s *Service) Resolve(ctx context.Context, driveID, path string) (Resolved, error) {
	drive, n, err := s.resolve(ctx, driveID, path)
	if err != nil {
		return Resolved{}, err
	}
	return Resolved{DriveID: drive, Node: n}, nil
}

// rootNodeID resolves the root node UUID for the given drive.
func (s *Service) rootNodeID(ctx context.Context, driveID string) (uuid.UUID, error) {
	d, err := s.Drive.GetByID(ctx, driveID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("vfs: %w", err)
	}
	if d == nil || d.RootNodeID() == nil {
		return uuid.Nil, ErrNotFound
	}
	return *d.RootNodeID(), nil
}

// checkAccess returns nil if the user has the given permission on the drive.
func (s *Service) checkAccess(ctx context.Context, userID, permission, driveID string) error {
	if s.Perm == nil {
		return nil
	}
	allowed, err := s.Perm.Check(ctx, userID, permission, "drive", driveID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPermission
	}
	return nil
}
