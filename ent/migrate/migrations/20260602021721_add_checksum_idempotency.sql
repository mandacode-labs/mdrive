-- Add checksum and idempotency_key columns to objects table
ALTER TABLE "objects" ADD COLUMN "checksum" character varying NULL;
ALTER TABLE "objects" ADD COLUMN "idempotency_key" character varying NULL;

-- Create index for idempotency key lookups
CREATE UNIQUE INDEX "object_system_id_idempotency_key" ON "objects" ("system_id", "idempotency_key");
