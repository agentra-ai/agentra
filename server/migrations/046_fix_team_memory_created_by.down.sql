ALTER TABLE team_memory
    ADD CONSTRAINT team_memory_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES agent(id);
