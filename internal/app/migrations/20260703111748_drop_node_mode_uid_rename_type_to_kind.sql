-- Drop POSIX permission/owner columns from the nodes table.
-- Permission checks are now owned by OpenFGA, so mode/uid/gid are
-- dead weight at the storage layer. Rename the type column to
-- kind to match the new NodeKind naming in the domain model.
ALTER TABLE "nodes" DROP CONSTRAINT IF EXISTS "nodes_type_check";
ALTER TABLE "nodes" RENAME COLUMN "type" TO "kind";
ALTER TABLE "nodes" DROP COLUMN "mode";
ALTER TABLE "nodes" DROP COLUMN "uid";
ALTER TABLE "nodes" DROP COLUMN "gid";
ALTER TABLE "nodes" ALTER COLUMN "kind" SET DEFAULT 'file';
ALTER TABLE "nodes" ADD CONSTRAINT "nodes_kind_check"
    CHECK ("kind" IN ('file', 'directory', 'symlink', 'object', 'mount'));
