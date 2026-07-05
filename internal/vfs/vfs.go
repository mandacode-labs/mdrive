package vfs

type VFS interface {
	NodeOperation
	DriveOperation
}

type vfs struct {
	NodeOperation
	DriveOperation
}

func NewVFS(nodeOp NodeOperation, driveOp DriveOperation) VFS {
	return &vfs{
		NodeOperation:  nodeOp,
		DriveOperation: driveOp,
	}
}
