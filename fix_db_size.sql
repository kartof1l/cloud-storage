-- Добавляем поле folder_size в таблицу files (если ещё нет)
ALTER TABLE files ADD COLUMN IF NOT EXISTS folder_size BIGINT DEFAULT 0;

-- Убираем NOT NULL у path в library_items
ALTER TABLE library_items ALTER COLUMN path DROP NOT NULL;

-- Добавляем поле avatar_url в users (если нет)
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(512);
