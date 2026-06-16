-- Create "nodes" table (POSIX-style inode)
-- One row per node. Filename and parent are NOT stored on the node;
-- they live in the parent directory's inline DirContent (JSON),
-- matching Linux where i_parent and i_name are absent from the inode.
CREATE TABLE "nodes" (
  "id" uuid NOT NULL,
  "create_time" timestamptz NOT NULL,
  "update_time" timestamptz NOT NULL,
  "drive_id" character varying(32) NOT NULL,
  "type" character varying NOT NULL DEFAULT 'file',
  "size" bigint NOT NULL DEFAULT 0,
  "nlink" bigint NOT NULL DEFAULT 1,
  "content" bytea NULL,
  "atime" timestamptz NOT NULL,
  "mtime" timestamptz NOT NULL,
  "ctime" timestamptz NOT NULL,
  "crtime" timestamptz NOT NULL,
  "flags" bigint NOT NULL DEFAULT 0,
  "revision" character varying(26) NOT NULL,
  PRIMARY KEY ("id")
);
-- Lookup by drive (most common access pattern: scope to a drive)
CREATE INDEX "node_drive_id" ON "nodes" ("drive_id");
-- Lookup by type within a drive (e.g., find all objects for GC)
CREATE INDEX "node_drive_id_type" ON "nodes" ("drive_id", "type");
