// ==================== ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ ====================
let currentUser = null;
let usersList = [];
let logsList = [];
let currentUsersPage = 1;
let currentLogsPage = 1;
let itemsPerPage = 20;
let selectedUserEmail = null;

// ==================== ИНИЦИАЛИЗАЦИЯ ====================
async function init() {
    const token = getToken();
    if (!token) {
        window.location.href = '/login';
        return;
    }
    
    await loadUserInfo();
    await checkAdminAccess();
    await loadUsers();
    await loadLogs();
}

async function loadUserInfo() {
    const token = getToken();
    try {
        const res = await fetch('/api/user/me', { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) {
            currentUser = await res.json();
            document.getElementById('userName').textContent = `${currentUser.first_name || ''} ${currentUser.last_name || ''}`.trim() || 'Пользователь';
            document.getElementById('userEmail').textContent = currentUser.email || '';
            if (currentUser.avatar_url) {
                document.getElementById('userAvatar').innerHTML = `<img src="${currentUser.avatar_url}">`;
            }
        }
    } catch(e) { console.error(e); }
}

async function checkAdminAccess() {
    const token = getToken();
    try {
        const res = await fetch('/api/library/admins', { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) {
            const admins = await res.json();
            const isAdmin = admins.some(a => a.email === currentUser?.email);
            if (!isAdmin) {
                alert('Доступ запрещен. Только администраторы могут просматривать эту страницу.');
                window.location.href = '/dashboard.html';
            }
        }
    } catch(e) { console.error(e); }
}

// ==================== ПОЛЬЗОВАТЕЛИ ====================
async function loadUsers() {
    const token = getToken();
    try {
        const res = await fetch('/api/admin/users', { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) {
            usersList = await res.json();
            displayUsers();
        }
    } catch(e) { console.error(e); }
}

function displayUsers() {
    const searchTerm = document.getElementById('userSearch')?.value.toLowerCase() || '';
    const filteredUsers = usersList.filter(u => u.email.toLowerCase().includes(searchTerm));
    
    const start = (currentUsersPage - 1) * itemsPerPage;
    const end = start + itemsPerPage;
    const pageUsers = filteredUsers.slice(start, end);
    
    const container = document.getElementById('usersList');
    if (!container) return;
    
    if (pageUsers.length === 0) {
        container.innerHTML = '<div class="empty-state">Нет пользователей</div>';
        return;
    }
    
    container.innerHTML = `
        <table class="users-table">
            <thead>
                <tr>
                    <th>Email</th>
                    <th>Имя</th>
                    <th>Статус</th>
                    <th>Использовано</th>
                    <th>Лимит</th>
                    <th>Действия</th>
                </tr>
            </thead>
            <tbody>
                ${pageUsers.map(user => `
                    <tr>
                        <td>${escapeHtml(user.email)}</td>
                        <td>${escapeHtml(user.first_name || '')} ${escapeHtml(user.last_name || '')}</td>
                        <td>
                            <span class="user-status ${user.is_blocked ? 'blocked' : 'active'}">
                                ${user.is_blocked ? '🔒 Заблокирован' : '✅ Активен'}
                            </span>
                            ${user.is_admin ? '<span class="user-status admin">👑 Админ</span>' : ''}
                        </td>
                        <td>${formatFileSize(user.storage_used || 0)}</td>
                        <td>
                            <div class="storage-limit">
                                <input type="number" id="limit_${user.id}" value="${Math.round((user.storage_limit || 100 * 1024 * 1024) / (1024 * 1024))}" step="50" style="width: 70px;">
                                <button class="action-btn set-limit" onclick="setUserLimit('${user.id}')">💾</button>
                            </div>
                        </td>
                        <td>
                            <button class="action-btn view-logs" onclick="viewUserLogs('${user.email}', '${escapeHtml(user.first_name)} ${escapeHtml(user.last_name)}')">📋 Логи</button>
                            ${!user.is_admin ? `
                                ${user.is_blocked ? 
                                    `<button class="action-btn unblock" onclick="toggleUserBlock('${user.id}', false)">🔓 Разблокировать</button>` :
                                    `<button class="action-btn block" onclick="toggleUserBlock('${user.id}', true)">🔒 Заблокировать</button>`
                                }
                                <button class="action-btn set-limit" onclick="makeAdmin('${user.id}', '${escapeHtml(user.email)}')">👑 Сделать админом</button>
                            ` : '<span style="color: var(--accent-primary);">Администратор</span>'}
                        </td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;
    
    displayUsersPagination(filteredUsers.length);
}

// Добавить функцию для назначения администратора
async function makeAdmin(userId, userEmail) {
    if (!confirm(`Сделать пользователя ${userEmail} администратором?`)) return;
    
    const token = getToken();
    try {
        const res = await fetch('/api/library/admins', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ email: userEmail })
        });
        if (res.ok) {
            showToast(`Пользователь ${userEmail} назначен администратором`);
            await loadUsers();
            await loadAdmins();
        } else {
            const error = await res.json();
            alert('Ошибка: ' + error.error);
        }
    } catch(e) { console.error(e); }
}

function displayUsersPagination(total) {
    const totalPages = Math.ceil(total / itemsPerPage);
    const container = document.getElementById('usersPagination');
    if (!container) return;
    
    let html = '';
    for (let i = 1; i <= Math.min(totalPages, 10); i++) {
        html += `<button class="${i === currentUsersPage ? 'active' : ''}" onclick="goToUsersPage(${i})">${i}</button>`;
    }
    container.innerHTML = html;
}

function goToUsersPage(page) {
    currentUsersPage = page;
    displayUsers();
}

function searchUsers() {
    currentUsersPage = 1;
    displayUsers();
}

async function toggleUserBlock(userId, block) {
    const token = getToken();
    try {
        const res = await fetch(`/api/admin/users/${userId}/block`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ blocked: block })
        });
        if (res.ok) {
            showToast(block ? 'Пользователь заблокирован' : 'Пользователь разблокирован');
            await loadUsers();
        } else {
            const error = await res.json();
            alert('Ошибка: ' + error.error);
        }
    } catch(e) { console.error(e); }
}

async function setUserLimit(userId) {
    const input = document.getElementById(`limit_${userId}`);
    const limit = parseInt(input.value);
    if (isNaN(limit) || limit < 0) {
        alert('Введите корректный лимит');
        return;
    }
    
    const token = getToken();
    try {
        const res = await fetch(`/api/admin/users/${userId}/limit`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ storage_limit: limit * 1024 * 1024 })
        });
        if (res.ok) {
            showToast(`Лимит установлен: ${limit} MB`);
            await loadUsers();
        } else {
            const error = await res.json();
            alert('Ошибка: ' + error.error);
        }
    } catch(e) { console.error(e); }
}

// ==================== ЛОГИ ====================
async function loadLogs() {
    const token = getToken();
    try {
        const res = await fetch('/api/admin/logs', { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) {
            logsList = await res.json();
            displayLogs();
        }
    } catch(e) { console.error(e); }
}

function displayLogs() {
    let filteredLogs = [...logsList];
    
    const searchTerm = document.getElementById('logSearch')?.value.toLowerCase();
    if (searchTerm) {
        filteredLogs = filteredLogs.filter(l => 
            l.user_email?.toLowerCase().includes(searchTerm) ||
            l.entity_name?.toLowerCase().includes(searchTerm) ||
            l.action?.toLowerCase().includes(searchTerm)
        );
    }
    
    const actionFilter = document.getElementById('logAction')?.value;
    if (actionFilter) {
        filteredLogs = filteredLogs.filter(l => l.action === actionFilter);
    }
    
    const dateFilter = document.getElementById('logDate')?.value;
    if (dateFilter) {
        filteredLogs = filteredLogs.filter(l => l.created_at?.startsWith(dateFilter));
    }
    
    const start = (currentLogsPage - 1) * itemsPerPage;
    const end = start + itemsPerPage;
    const pageLogs = filteredLogs.slice(start, end);
    
    const container = document.getElementById('logsList');
    if (!container) return;
    
    if (pageLogs.length === 0) {
        container.innerHTML = '<div class="empty-state">Нет логов</div>';
        return;
    }
    
    container.innerHTML = `
        <table class="logs-table">
            <thead>
                <tr>
                    <th>Дата</th>
                    <th>Пользователь</th>
                    <th>Действие</th>
                    <th>Объект</th>
                    <th>Детали</th>
                    <th>IP</th>
                </tr>
            </thead>
            <tbody>
                ${pageLogs.map(log => `
                    <tr>
                        <td>${new Date(log.created_at).toLocaleString()}</td>
                        <td>${escapeHtml(log.user_email)}</td>
                        <td>${getActionIcon(log.action)} ${escapeHtml(log.action)}</td>
                        <td>${escapeHtml(log.entity_type)}: ${escapeHtml(log.entity_name || log.entity_id || '')}</td>
                        <td>${escapeHtml(log.details || '')}</td>
                        <td>${escapeHtml(log.ip_address || '')}</td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;
    
    displayLogsPagination(filteredLogs.length);
}

function getActionIcon(action) {
    const icons = {
        'upload': '⬆️',
        'download': '⬇️',
        'delete': '🗑️',
        'create_folder': '📁',
        'library_upload': '📚⬆️',
        'library_delete': '📚🗑️'
    };
    return icons[action] || '📋';
}

function displayLogsPagination(total) {
    const totalPages = Math.ceil(total / itemsPerPage);
    const container = document.getElementById('logsPagination');
    if (!container) return;
    
    let html = '';
    for (let i = 1; i <= Math.min(totalPages, 10); i++) {
        html += `<button class="${i === currentLogsPage ? 'active' : ''}" onclick="goToLogsPage(${i})">${i}</button>`;
    }
    container.innerHTML = html;
}

function goToLogsPage(page) {
    currentLogsPage = page;
    displayLogs();
}

function searchLogs() {
    currentLogsPage = 1;
    displayLogs();
}

function filterLogs() {
    currentLogsPage = 1;
    displayLogs();
}

// ==================== ЛОГИ ПОЛЬЗОВАТЕЛЯ ====================
async function viewUserLogs(email, name) {
    selectedUserEmail = email;
    document.getElementById('modalUserName').textContent = name || email;
    document.getElementById('modalUserEmail').textContent = email;
    
    const token = getToken();
    try {
        const userRes = await fetch(`/api/admin/users/by-email/${encodeURIComponent(email)}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (userRes.ok) {
            const user = await userRes.json();
            document.getElementById('modalUserStatus').innerHTML = user.is_blocked ? '🔒 Заблокирован' : '✅ Активен';
            document.getElementById('modalUserStorage').textContent = formatFileSize(user.storage_used || 0);
            document.getElementById('modalUserLimit').textContent = formatFileSize(user.storage_limit || 100 * 1024 * 1024);
        }
        
        const logsRes = await fetch(`/api/admin/logs/user/${encodeURIComponent(email)}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (logsRes.ok) {
            const userLogs = await logsRes.json();
            const container = document.getElementById('userLogsList');
            
            if (userLogs.length === 0) {
                container.innerHTML = '<div class="empty-state">Нет действий</div>';
            } else {
                container.innerHTML = `
                    <table class="logs-table">
                        <thead>
                            <tr>
                                <th>Дата</th>
                                <th>Действие</th>
                                <th>Объект</th>
                                <th>Детали</th>
                                <th>IP</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${userLogs.map(log => `
                                <tr>
                                    <td>${new Date(log.created_at).toLocaleString()}</td>
                                    <td>${getActionIcon(log.action)} ${escapeHtml(log.action)}</td>
                                    <td>${escapeHtml(log.entity_type)}: ${escapeHtml(log.entity_name || log.entity_id || '')}</td>
                                    <td>${escapeHtml(log.details || '')}</td>
                                    <td>${escapeHtml(log.ip_address || '')}</td>
                                </tr>
                            `).join('')}
                        </tbody>
                    </table>
                `;
            }
        }
    } catch(e) { console.error(e); }
    
    document.getElementById('userLogsModal').style.display = 'flex';
}

function closeUserLogsModal() {
    document.getElementById('userLogsModal').style.display = 'none';
    selectedUserEmail = null;
}

// ==================== НАСТРОЙКИ ====================
async function updateDefaultLimit() {
    const limit = parseInt(document.getElementById('defaultStorageLimit').value);
    const token = getToken();
    try {
        const res = await fetch('/api/admin/settings/default-limit', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ default_limit: limit * 1024 * 1024 })
        });
        if (res.ok) {
            showToast(`Лимит по умолчанию: ${limit} MB`);
        }
    } catch(e) { console.error(e); }
}

async function updateMaxFileSize() {
    const size = parseInt(document.getElementById('maxFileSize').value);
    const token = getToken();
    try {
        const res = await fetch('/api/admin/settings/max-file-size', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ max_file_size: size * 1024 * 1024 })
        });
        if (res.ok) {
            showToast(`Максимальный размер файла: ${size} MB`);
        }
    } catch(e) { console.error(e); }
}

async function cleanInactiveUsers() {
    const days = parseInt(document.getElementById('inactiveDays').value);
    if (!confirm(`Удалить пользователей, неактивных более ${days} дней?`)) return;
    
    const token = getToken();
    try {
        const res = await fetch('/api/admin/users/clean-inactive', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ days: days })
        });
        if (res.ok) {
            const data = await res.json();
            showToast(`Удалено ${data.deleted_count} пользователей`);
            await loadUsers();
        }
    } catch(e) { console.error(e); }
}

// ==================== ВКЛАДКИ ====================
function switchTab(tab) {
    // Скрываем все вкладки
    document.querySelectorAll('.tab-content').forEach(t => t.style.display = 'none');
    
    // Показываем выбранную
    document.getElementById(`${tab}Tab`).style.display = 'block';
    
    // Обновляем активный класс у табов
    document.querySelectorAll('.admin-tab').forEach(t => t.classList.remove('active'));
    event.target.classList.add('active');
    
    // Загружаем данные при переключении
    if (tab === 'users') {
        loadUsers();
    } else if (tab === 'admins') {
        loadAdmins();
    } else if (tab === 'logs') {
        loadLogs();
    } else if (tab === 'stats') {
        loadStats();
    }
}
async function loadStats() {
    const token = getToken();
    try {
        // Общая статистика пользователей
        const usersRes = await fetch('/api/admin/users', { headers: { 'Authorization': `Bearer ${token}` } });
        if (usersRes.ok) {
            const users = await usersRes.json();
            document.getElementById('totalUsers').textContent = users.length;
            document.getElementById('activeUsers').textContent = users.filter(u => !u.is_blocked).length;
        }
        
        // Статистика файлов
        const filesRes = await fetch('/api/files?page=1&limit=1', { headers: { 'Authorization': `Bearer ${token}` } });
        if (filesRes.ok) {
            const data = await filesRes.json();
            document.getElementById('totalFiles').textContent = data.total || 0;
        }
        
        // Статистика хранилища
        const statsRes = await fetch('/api/storage/stats', { headers: { 'Authorization': `Bearer ${token}` } });
        if (statsRes.ok) {
            const stats = await statsRes.json();
            document.getElementById('totalStorage').textContent = formatFileSize(stats.total_size || 0);
        }
    } catch(e) { console.error(e); }
}
// ==================== УПРАВЛЕНИЕ АДМИНИСТРАТОРАМИ ====================
let adminsList = [];

async function loadAdmins() {
    const token = getToken();
    try {
        const res = await fetch('/api/library/admins', { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) {
            adminsList = await res.json();
            displayAdmins();
        }
    } catch(e) { console.error(e); }
}

function displayAdmins() {
    const container = document.getElementById('adminsList');
    if (!container) return;
    
    if (!adminsList || adminsList.length === 0) {
        container.innerHTML = '<div class="empty-state">Нет администраторов</div>';
        return;
    }
    
    container.innerHTML = `
        <table class="users-table">
            <thead>
                <tr>
                    <th>Email</th>
                    <th>Дата добавления</th>
                    <th>Действия</th>
                </tr>
            </thead>
            <tbody>
                ${adminsList.map(admin => `
                    <tr>
                        <td>${escapeHtml(admin.email)} ${admin.email === currentUser?.email ? ' 👑 (Вы)' : ''}</td>
                        <td>${new Date(admin.added_at).toLocaleString()}</td>
                        <td>
                            ${admin.email !== currentUser?.email ? 
                                `<button class="action-btn block" onclick="removeAdminFromList('${escapeHtml(admin.email)}')">🗑️ Удалить</button>` : 
                                '<span style="color: var(--text-muted);">Нельзя удалить себя</span>'
                            }
                        </td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;
}

async function addNewAdmin() {
    const email = document.getElementById('newAdminEmail').value.trim();
    if (!email) {
        showToast('Введите email администратора');
        return;
    }
    
    const token = getToken();
    try {
        const res = await fetch('/api/library/admins', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ email })
        });
        if (res.ok) {
            document.getElementById('newAdminEmail').value = '';
            showToast('Администратор добавлен');
            await loadAdmins();
        } else {
            const error = await res.json();
            alert('Ошибка: ' + error.error);
        }
    } catch(e) { console.error(e); }
}

async function removeAdminFromList(email) {
    if (!confirm(`Удалить администратора ${email}?`)) return;
    const token = getToken();
    try {
        const res = await fetch(`/api/library/admins/${encodeURIComponent(email)}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (res.ok) {
            showToast('Администратор удален');
            await loadAdmins();
            await loadUsers(); // Обновляем список пользователей, чтобы обновить статус
        }
    } catch(e) { console.error(e); }
}
// ==================== ИНИЦИАЛИЗАЦИЯ ====================
document.addEventListener('DOMContentLoaded', init);