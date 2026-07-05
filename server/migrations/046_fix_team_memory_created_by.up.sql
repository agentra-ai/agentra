-- The original FK pointed at agent(id) but created_by stores the user id
-- (the HTTP caller's X-User-ID). Drop the broken FK; created_by is now an
-- unenforced UUID. A backfill can populate it from audit logs if needed.
ALTER TABLE team_memory DROP CONSTRAINT team_memory_created_by_fkey;
