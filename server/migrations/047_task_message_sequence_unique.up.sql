CREATE UNIQUE INDEX uq_task_message_task_id_seq
    ON task_message(task_id, seq);
