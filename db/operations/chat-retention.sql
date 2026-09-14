BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '20s';
CREATE EXTENSION IF NOT EXISTS pg_cron;
SELECT cron.schedule('bridgeyok-chat-retention', '17 3 * * *', 'SELECT bridgeyok.cleanup_chat_messages()');
COMMIT;
