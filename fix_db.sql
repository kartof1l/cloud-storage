-- Убираем NOT NULL у path в library_items (чтобы папки могли создаваться)
ALTER TABLE library_items ALTER COLUMN path DROP NOT NULL;

-- Убеждаемся, что avatar_url есть в таблице users
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(512);

-- Убеждаемся, что таблица library_admins существует
CREATE TABLE IF NOT EXISTS library_admins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    user_id UUID REFERENCES users(id),
    added_by UUID NOT NULL REFERENCES users(id),
    added_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
