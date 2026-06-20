-- Modify "drives" table
ALTER TABLE "drives" ADD COLUMN "deleted_at" timestamptz NULL;
-- Create index "drive_deleted_at" to table: "drives"
CREATE INDEX "drive_deleted_at" ON "drives" ("deleted_at");
