-- Существующие таблицы (оставляем как есть)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    is_email_verified BOOLEAN DEFAULT FALSE,
    verification_code VARCHAR(10),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Добавляем avatar_url если нет
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(512);

-- Таблица files с folder_size
CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    path VARCHAR(1024),
    size BIGINT DEFAULT 0,
    folder_size BIGINT DEFAULT 0,
    mime_type VARCHAR(255),
    is_folder BOOLEAN DEFAULT FALSE,
    parent_folder_id UUID REFERENCES files(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_parent_folder FOREIGN KEY (parent_folder_id) REFERENCES files(id) ON DELETE CASCADE
);

-- Индексы для files
CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_files_parent_folder ON files(parent_folder_id);
CREATE INDEX IF NOT EXISTS idx_files_is_folder ON files(is_folder);
CREATE INDEX IF NOT EXISTS idx_files_folder_size ON files(folder_size) WHERE folder_size > 0;

-- Таблица library_items (снимаем NOT NULL с path для папок)
CREATE TABLE IF NOT EXISTS library_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    mime_type VARCHAR(255),
    size BIGINT DEFAULT 0,
    path VARCHAR(1024), -- убираем NOT NULL для папок
    is_folder BOOLEAN DEFAULT FALSE,
    parent_id UUID REFERENCES library_items(id) ON DELETE CASCADE,
    version INT DEFAULT 1,
    created_by UUID NOT NULL REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_parent_library FOREIGN KEY (parent_id) REFERENCES library_items(id) ON DELETE CASCADE
);

-- Таблица library_versions
CREATE TABLE IF NOT EXISTS library_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES library_items(id) ON DELETE CASCADE,
    version INT NOT NULL,
    size BIGINT,
    path VARCHAR(1024),
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Таблица library_admins
CREATE TABLE IF NOT EXISTS library_admins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    user_id UUID REFERENCES users(id),
    added_by UUID NOT NULL REFERENCES users(id),
    added_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Таблица audit_logs (добавляем, если нет)
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_email VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID,
    entity_name VARCHAR(255),
    details TEXT,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Индексы для производительности
CREATE INDEX IF NOT EXISTS idx_library_items_parent ON library_items(parent_id);
CREATE INDEX IF NOT EXISTS idx_library_items_name ON library_items(name);
CREATE INDEX IF NOT EXISTS idx_library_items_deleted ON library_items(deleted_at);
CREATE INDEX IF NOT EXISTS idx_library_versions_item ON library_versions(item_id);
CREATE INDEX IF NOT EXISTS idx_library_admins_email ON library_admins(email);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at DESC);

-- Функция для автоматического обновления folder_size (триггер)
CREATE OR REPLACE FUNCTION update_parent_folder_size()
RETURNS TRIGGER AS $$
BEGIN
    -- При вставке/обновлении/удалении файла обновляем размер родительской папки
    IF TG_OP = 'DELETE' THEN
        IF OLD.parent_folder_id IS NOT NULL THEN
            PERFORM update_folder_size_recursive(OLD.parent_folder_id);
        END IF;
        RETURN OLD;
    ELSE
        IF NEW.parent_folder_id IS NOT NULL THEN
            PERFORM update_folder_size_recursive(NEW.parent_folder_id);
        END IF;
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Триггер для автоматического обновления размеров (опционально)
DROP TRIGGER IF EXISTS trigger_update_folder_size ON files;
CREATE TRIGGER trigger_update_folder_size
    AFTER INSERT OR UPDATE OF size, parent_folder_id OR DELETE ON files
    FOR EACH ROW
    EXECUTE FUNCTION update_parent_folder_size();

-- Таблица для мероприятий расписания
-- Таблица для задач (вместо schedule_events)
DROP TABLE IF EXISTS schedule_events;
CREATE TABLE IF NOT EXISTS schedule_tasks (
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    due_date DATE,
    due_time VARCHAR(10),
    priority VARCHAR(20) DEFAULT 'medium',
    completed BOOLEAN DEFAULT FALSE,
    created_by VARCHAR(36) REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_schedule_tasks_date ON schedule_tasks(due_date);
CREATE INDEX IF NOT EXISTS idx_schedule_tasks_created_by ON schedule_tasks(created_by);
CREATE INDEX IF NOT EXISTS idx_schedule_tasks_completed ON schedule_tasks(completed);