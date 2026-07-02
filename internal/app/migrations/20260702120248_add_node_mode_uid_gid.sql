-- Add POSIX permission/owner columns to the nodes table.
-- These were added to the ent schema in a prior commit but the
-- corresponding Atlas migration was never generated, so the
-- production nodes table is missing them. Drive creation fails
-- at the root-directory INSERT with
--   pq: column "mode" of relation "nodes" does not exist
-- until this migration lands.
ALTER TABLE "nodes" ADD COLUMN "mode" bigint NOT NULL DEFAULT 420;
ALTER TABLE "nodes" ADD COLUMN "uid" character varying NOT NULL DEFAULT '';
ALTER TABLE "nodes" ADD COLUMN "gid" character varying NOT NULL DEFAULT '';
