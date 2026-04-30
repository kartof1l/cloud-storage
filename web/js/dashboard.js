// ==================== ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ ====================
let currentFolderId = null;
let currentFolderPath = [{ id: null, name: 'Главная' }];
let historyStack = [];
let isLibrary = false;
let currentUser = null;
let draggedItem = null;
let backNavTimer = null;
let isDraggingToBack = false;
let moveModeActive = false;
let moveModeItem = null;
let isReturningWithItem = false;
let currentAdminList = [];
let isCurrentUserAdmin = false;
let longPressTimer = null;
let currentLongPressCard = null;
let touchMoveTarget = null;
let isTouchDevice = ('ontouchstart' in window) || (navigator.maxTouchPoints > 0);
window.isCurrentUserAdmin = false;

// ==================== ПОЛЬЗОВАТЕЛЬ ====================
async function loadUserInfo() {
    const token = getToken();
    if (!token) return;
    try {
        const res = await fetch('/api/user/me', { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) {
            currentUser = await res.json();
            localStorage.setItem('user', JSON.stringify(currentUser));
            document.getElementById('userName').textContent = `${currentUser.first_name || ''} ${currentUser.last_name || ''}`.trim() || 'Пользователь';
            document.getElementById('userEmail').textContent = currentUser.email || '';
            if (currentUser.avatar_url) {
                document.getElementById('userAvatar').innerHTML = `<img src="${currentUser.avatar_url}">`;
                document.getElementById('avatarPreview').innerHTML = `<img src="${currentUser.avatar_url}">`;
            }
        }
    } catch(e) { console.error(e); }
}

async function checkIfUserIsAdmin() {
    const token = getToken();
    if (!token) return false;
    try {
        const res = await fetch('/api/library/admins', { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) {
            const admins = await res.json();
            isCurrentUserAdmin = admins.some(admin => admin.email === currentUser?.email);
            window.isCurrentUserAdmin = isCurrentUserAdmin;
            const adminNav = document.getElementById('navAdmin');
            if (adminNav) adminNav.style.display = isCurrentUserAdmin ? 'flex' : 'none';
            const quickAddBlock = document.getElementById('quickAddBlock');
            if (quickAddBlock) quickAddBlock.style.display = isCurrentUserAdmin ? 'block' : 'none';
            return isCurrentUserAdmin;
        }
    } catch(e) { console.error(e); }
    isCurrentUserAdmin = false;
    window.isCurrentUserAdmin = false;
    const adminNav = document.getElementById('navAdmin');
    if (adminNav) adminNav.style.display = 'none';
    return false;
}

function showProfileModal() {
    document.getElementById('editFirstName').value = currentUser?.first_name || '';
    document.getElementById('editLastName').value = currentUser?.last_name || '';
    document.getElementById('profileModal').style.display = 'flex';
}

function closeProfileModal() { document.getElementById('profileModal').style.display = 'none'; }

async function saveProfile() {
    const token = getToken();
    const firstName = document.getElementById('editFirstName').value;
    const lastName = document.getElementById('editLastName').value;
    try {
        const res = await fetch('/api/user/me', {
            method: 'PUT',
            headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
            body: JSON.stringify({ first_name: firstName, last_name: lastName })
        });
        if (res.ok) {
            if (currentUser) { currentUser.first_name = firstName; currentUser.last_name = lastName; localStorage.setItem('user', JSON.stringify(currentUser)); }
            document.getElementById('userName').textContent = `${firstName} ${lastName}`.trim() || 'Пользователь';
            closeProfileModal();
        }
    } catch(e) { console.error(e); }
}

document.getElementById('avatarInput')?.addEventListener('change', async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    const token = getToken();
    const formData = new FormData();
    formData.append('avatar', file);
    try {
        const res = await fetch('/api/user/avatar', {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${token}` },
            body: formData
        });
        if (res.ok) {
            const data = await res.json();
            document.getElementById('userAvatar').innerHTML = `<img src="${data.avatar_url}">`;
            document.getElementById('avatarPreview').innerHTML = `<img src="${data.avatar_url}">`;
            if (currentUser) currentUser.avatar_url = data.avatar_url;
        }
    } catch(e) { console.error(e); }
});

// ==================== АДМИНИСТРИРОВАНИЕ ====================
async function loadAdmins() {
    const token = getToken();
    try {
        const res = await fetch('/api/library/admins', { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) { currentAdminList = await res.json(); displayAdminList(); }
    } catch(e) { console.error(e); }
}

function displayAdminList() {
    const container = document.getElementById('adminListContainer');
    if (!container) return;
    if (!currentAdminList || currentAdminList.length === 0) {
        container.innerHTML = '<div style="text-align:center;padding:20px;">Нет администраторов</div>';
        return;
    }
    container.innerHTML = currentAdminList.map(admin => `
        <div class="admin-item">
            <span>${escapeHtml(admin.email)}</span>
            <button class="admin-remove" onclick="removeAdmin('${escapeHtml(admin.email)}')">Удалить</button>
        </div>
    `).join('');
}

async function addAdmin() {
    const email = document.getElementById('newAdminEmail').value.trim();
    if (!email) return;
    const token = getToken();
    try {
        const res = await fetch('/api/library/admins', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ email })
        });
        if (res.ok) { document.getElementById('newAdminEmail').value = ''; await loadAdmins(); showToast('Администратор добавлен'); }
        else { const error = await res.json(); alert('Ошибка: ' + error.error); }
    } catch(e) { console.error(e); }
}

async function removeAdmin(email) {
    if (!confirm(`Удалить администратора ${email}?`)) return;
    const token = getToken();
    try {
        const res = await fetch(`/api/library/admins/${encodeURIComponent(email)}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (res.ok) { await loadAdmins(); showToast('Администратор удален'); }
    } catch(e) { console.error(e); }
}

function showAdminModal() { loadAdmins(); document.getElementById('adminModal').style.display = 'flex'; }
function closeAdminModal() { document.getElementById('adminModal').style.display = 'none'; }

// ==================== ПЕРЕМЕЩЕНИЕ ====================
async function executeMove(itemId, itemType, targetFolderId) {
    const token = getToken();
    const url = itemType === 'folder' ? `/api/folders/${itemId}/move` : `/api/files/${itemId}/move`;
    try {
        const response = await fetch(url, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ parent_folder_id: targetFolderId })
        });
        if (response.ok) { deactivateMoveMode(); loadContent(currentFolderId); return true; }
        else { const error = await response.json(); alert('Ошибка: ' + (error.error || 'Неизвестная ошибка')); return false; }
    } catch (error) { console.error('Move error:', error); alert('Ошибка соединения'); return false; }
}

function activateMoveMode(id, type, name) {
    if (moveModeActive) deactivateMoveMode();
    moveModeActive = true;
    moveModeItem = { id, type, name };
    document.querySelectorAll('.bubble-card, .file-card').forEach(card => {
        if (card.dataset.id !== id) card.classList.add('move-target');
    });
    const backZone = document.getElementById('backNavZone');
    backZone.classList.remove('hidden');
    backZone.classList.add('move-target');
    showToast(`Перемещение: ${name} — нажмите на папку`, 1500);
}

function deactivateMoveMode() {
    moveModeActive = false;
    moveModeItem = null;
    document.querySelectorAll('.move-target').forEach(el => el.classList.remove('move-target'));
    const backZone = document.getElementById('backNavZone');
    backZone.classList.add('hidden');
    backZone.classList.remove('move-target');
}

function handleItemClickForMove(id, name) {
    if (!moveModeActive) return false;
    if (moveModeItem.id === id) { showToast('Нельзя переместить в самого себя'); return false; }
    executeMove(moveModeItem.id, moveModeItem.type, id);
    return true;
}

// ==================== DRAG & DROP ====================
function startDrag(e, id, type, name) {
    if (isTouchDevice) return;
    draggedItem = { id, type, name };
    e.dataTransfer.setData('text/plain', JSON.stringify({ id, type, name }));
    e.dataTransfer.effectAllowed = 'move';
    const element = e.target.closest('.bubble-card, .file-card');
    if (element) element.classList.add('dragging');
    const backZone = document.getElementById('backNavZone');
    if (currentFolderPath.length > 1) backZone.classList.remove('hidden');
}

function endDrag(e) {
    document.querySelectorAll('.dragging').forEach(el => el.classList.remove('dragging'));
    document.querySelectorAll('.drag-over').forEach(el => el.classList.remove('drag-over'));
    if (backNavTimer) clearTimeout(backNavTimer);
    isDraggingToBack = false;
    const backZone = document.getElementById('backNavZone');
    backZone.classList.remove('drag-over', 'active');
    backZone.classList.add('hidden');
    draggedItem = null;
}

function onBackZoneDragOver(e) {
    e.preventDefault();
    if (!draggedItem) return;
    const backZone = document.getElementById('backNavZone');
    backZone.classList.add('drag-over');
    if (!isDraggingToBack) {
        isDraggingToBack = true;
        backNavTimer = setTimeout(() => {
            if (draggedItem && currentFolderPath.length > 1) { isReturningWithItem = true; goBack(); }
            backZone.classList.remove('drag-over');
        }, 800);
    }
}

function onBackZoneDragLeave(e) {
    const backZone = document.getElementById('backNavZone');
    backZone.classList.remove('drag-over');
    if (backNavTimer) { clearTimeout(backNavTimer); backNavTimer = null; }
    isDraggingToBack = false;
}

function onBackZoneDrop(e) {
    e.preventDefault();
    if (backNavTimer) clearTimeout(backNavTimer);
    isDraggingToBack = false;
    const backZone = document.getElementById('backNavZone');
    backZone.classList.remove('drag-over');
    if (isReturningWithItem) { isReturningWithItem = false; showToast(`Объект в руке, выберите папку`); }
    else if (draggedItem) { executeMove(draggedItem.id, draggedItem.type, currentFolderPath[currentFolderPath.length - 2]?.id || null); }
    endDrag(e);
}

// ==================== МОБИЛЬНЫЕ ЖЕСТЫ ====================
function onTouchStart(e, id, type, name) {
    const card = e.target.closest('.bubble-card, .file-card');
    if (!card) return;
    if (e.target.closest('.back-action-btn')) return;
    e.preventDefault();
    currentLongPressCard = { id, type, name, card };
    longPressTimer = setTimeout(() => {
        if (currentLongPressCard) { currentLongPressCard.card.classList.add('tapped'); activateMoveMode(id, type, name); showToast(`Удерживайте и перетащите на папку`); }
    }, 500);
}

function onTouchMove(e) {
    if (!moveModeActive || !moveModeItem) return;
    e.preventDefault();
    const touch = e.touches[0];
    const elementUnderFinger = document.elementFromPoint(touch.clientX, touch.clientY);
    const backZone = document.getElementById('backNavZone');
    const targetCard = elementUnderFinger?.closest('.bubble-card, .file-card');
    document.querySelectorAll('.bubble-card, .file-card').forEach(card => card.classList.remove('drag-over'));
    backZone.classList.remove('drag-over');
    if (targetCard && targetCard.dataset.id !== moveModeItem.id) { targetCard.classList.add('drag-over'); touchMoveTarget = targetCard.dataset.id; }
    else if (elementUnderFinger === backZone || backZone.contains(elementUnderFinger)) { backZone.classList.add('drag-over'); touchMoveTarget = 'back'; }
    else { touchMoveTarget = null; }
}

function onTouchEnd(e) {
    if (longPressTimer) { clearTimeout(longPressTimer); longPressTimer = null; }
    if (moveModeActive && moveModeItem) {
        if (touchMoveTarget && touchMoveTarget !== moveModeItem.id && touchMoveTarget !== 'back') { executeMove(moveModeItem.id, moveModeItem.type, touchMoveTarget); }
        else if (touchMoveTarget === 'back') { if (currentFolderPath.length > 1) { goBack(); showToast(`Объект в руке, выберите папку`); } }
        else { deactivateMoveMode(); }
    }
    if (currentLongPressCard) { setTimeout(() => { if (currentLongPressCard.card) currentLongPressCard.card.classList.remove('tapped'); }, 100); currentLongPressCard = null; }
    touchMoveTarget = null;
}

// ==================== ЗАГРУЗКА КОНТЕНТА ====================
async function loadContent(folderId = null) {
    const token = getToken();
    if (!token) return;
    let url = folderId ? `/api/files?folder_id=${folderId}&page=1&limit=100` : '/api/files?page=1&limit=100';
    try {
        const res = await fetch(url, { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) { const data = await res.json(); displayContent(data.files || data || []); updateStorageUsed(); }
    } catch(e) { console.error(e); }
}

function displayContent(items) {
    const folders = items.filter(f => f.is_folder);
    const files = items.filter(f => !f.is_folder);
    const bubblesSection = document.getElementById('bubblesSection');
    const filesSection = document.getElementById('filesSection');
    const emptyState = document.getElementById('emptyState');
    const backButton = document.getElementById('backButton');

    if (currentFolderPath.length <= 1) { backButton.style.opacity = '0.5'; backButton.style.pointerEvents = 'none'; }
    else { backButton.style.opacity = '1'; backButton.style.pointerEvents = 'auto'; }

    if (items.length === 0) { bubblesSection.style.display = 'none'; filesSection.style.display = 'none'; emptyState.style.display = 'block'; return; }
    emptyState.style.display = 'none';
    if (folders.length > 0) { bubblesSection.style.display = 'block'; displayBubbles(folders); }
    else bubblesSection.style.display = 'none';
    if (files.length > 0) { filesSection.style.display = 'block'; displayFiles(files); }
    else filesSection.style.display = 'none';
}

function displayBubbles(folders) {
    const container = document.getElementById('bubblesContainer');
    container.innerHTML = folders.map((f, i) => `
        <div class="bubble-card" data-id="${f.id}" data-name="${escapeHtml(f.name)}" draggable="${!isTouchDevice}">
            <div class="card-inner">
                <div class="card-front">
                    <div class="bubble-icon" style="background: ${getGradient(i)}">📁</div>
                    <div class="bubble-name">${escapeHtml(f.name)}</div>
                    <div class="bubble-size">${formatFileSize(f.folder_size || f.size || 0)}</div>
                </div>
                <div class="card-back">
                    <div class="back-actions">
                        <div class="back-action-btn" onclick="event.stopPropagation(); openFolder('${f.id}', '${escapeHtml(f.name)}')">📂 <span>Открыть</span></div>
                        <div class="back-action-btn" onclick="event.stopPropagation(); startRename('${f.id}', true, '${escapeHtml(f.name)}')">✏️ <span>Ред.</span></div>
                        <div class="back-action-btn" onclick="event.stopPropagation(); deleteItem('${f.id}', true)">🗑️ <span>Удалить</span></div>
                        <div class="back-action-btn move-btn" data-id="${f.id}" data-name="${escapeHtml(f.name)}" data-type="folder">🔄 <span>Перем.</span></div>
                        ${isCurrentUserAdmin ? `<div class="back-action-btn library-btn" onclick="event.stopPropagation(); moveToLibrary('${f.id}', '${escapeHtml(f.name)}', true)">📚 <span>В библ.</span></div>` : ''}
                    </div>
                </div>
            </div>
        </div>
    `).join('');

    document.querySelectorAll('.bubble-card').forEach(card => {
        const folderId = card.dataset.id;
        const folderName = card.dataset.name;
        const front = card.querySelector('.card-front');
        front.addEventListener('click', (e) => {
            if (e.target.closest('.back-action-btn')) return;
            if (moveModeActive) handleItemClickForMove(folderId, folderName);
            else openFolder(folderId, folderName);
        });
        if (isTouchDevice) {
            card.addEventListener('touchstart', (e) => onTouchStart(e, folderId, 'folder', folderName), { passive: false });
            card.addEventListener('touchmove', onTouchMove, { passive: false });
            card.addEventListener('touchend', onTouchEnd);
        }
        const moveBtn = card.querySelector('.move-btn');
        if (moveBtn) moveBtn.addEventListener('click', (e) => { e.stopPropagation(); activateMoveMode(folderId, 'folder', folderName); });
    });

    if (!isTouchDevice) {
        document.querySelectorAll('.bubble-card').forEach(card => {
            card.addEventListener('dragstart', (e) => startDrag(e, card.dataset.id, 'folder', card.dataset.name));
            card.addEventListener('dragend', endDrag);
            card.addEventListener('dragover', (e) => e.preventDefault());
            card.addEventListener('drop', (e) => { e.preventDefault(); if (draggedItem) { executeMove(draggedItem.id, draggedItem.type, card.dataset.id); endDrag(e); } });
        });
    }
}

function displayFiles(files) {
    const container = document.getElementById('fileGrid');
    const token = getToken();
    container.innerHTML = files.map((file, i) => {
        const isImage = file.mime_type?.startsWith('image/');
        const previewUrl = `/api/files/${file.id}/download?token=${encodeURIComponent(token)}`;
        return `
            <div class="file-card" data-id="${file.id}" data-name="${escapeHtml(file.name)}" draggable="${!isTouchDevice}">
                <div class="card-inner">
                    <div class="card-front">
                        <div class="file-preview" style="background: ${getGradient(i)}">
                            ${isImage ? `<img src="${previewUrl}" style="width:100%;height:100%;object-fit:cover;border-radius:50%" onerror="this.parentElement.innerHTML='📄'">` : `<span>${getFileIcon(file.mime_type)}</span>`}
                        </div>
                        <div class="file-name">${escapeHtml(file.name)}</div>
                        <div class="file-meta">${formatFileSize(file.size)}</div>
                    </div>
                    <div class="card-back">
                        <div class="back-actions">
                            <div class="back-action-btn" onclick="event.stopPropagation(); downloadFile('${file.id}')">⬇️ <span>Скачать</span></div>
                            <div class="back-action-btn" onclick="event.stopPropagation(); startRename('${file.id}', false, '${escapeHtml(file.name)}')">✏️ <span>Ред.</span></div>
                            <div class="back-action-btn" onclick="event.stopPropagation(); deleteItem('${file.id}', false)">🗑️ <span>Удалить</span></div>
                            <div class="back-action-btn move-file-btn" data-id="${file.id}" data-name="${escapeHtml(file.name)}" data-type="file">🔄 <span>Перем.</span></div>
                            ${isCurrentUserAdmin ? `<div class="back-action-btn library-btn" onclick="event.stopPropagation(); moveToLibrary('${file.id}', '${escapeHtml(file.name)}', false)">📚 <span>В библ.</span></div>` : ''}
                        </div>
                    </div>
                </div>
            </div>
        `;
    }).join('');

    document.querySelectorAll('.file-card').forEach(card => {
        const fileId = card.dataset.id;
        const fileName = card.dataset.name;
        const front = card.querySelector('.card-front');
        front.addEventListener('click', (e) => {
            if (e.target.closest('.back-action-btn')) return;
            if (moveModeActive) handleItemClickForMove(fileId, fileName);
            else downloadFile(fileId);
        });
        if (isTouchDevice) {
            card.addEventListener('touchstart', (e) => onTouchStart(e, fileId, 'file', fileName), { passive: false });
            card.addEventListener('touchmove', onTouchMove, { passive: false });
            card.addEventListener('touchend', onTouchEnd);
        }
        if (!isTouchDevice) {
            card.addEventListener('dragstart', (e) => startDrag(e, fileId, 'file', fileName));
            card.addEventListener('dragend', endDrag);
            card.addEventListener('dragover', (e) => e.preventDefault());
            card.addEventListener('drop', (e) => { e.preventDefault(); if (draggedItem) { executeMove(draggedItem.id, draggedItem.type, currentFolderId); endDrag(e); } });
        }
    });

    document.querySelectorAll('.move-file-btn').forEach(btn => {
        btn.addEventListener('click', (e) => { e.stopPropagation(); activateMoveMode(btn.dataset.id, 'file', btn.dataset.name); });
    });
}

// ==================== НАВИГАЦИЯ ====================
function loadRootFiles() { isLibrary = false; currentFolderId = null; currentFolderPath = [{ id: null, name: 'Главная' }]; updateBreadcrumb(); loadContent(null); setActiveNav('navHome'); }
function loadLibrary() { window.location.href = '/library.html'; }
function setActiveNav(id) { document.querySelectorAll('.sidebar-menu a').forEach(a => a.classList.remove('active')); document.getElementById(id)?.classList.add('active'); }

function openFolder(id, name) { historyStack.push(currentFolderId); currentFolderId = id; currentFolderPath.push({ id, name }); updateBreadcrumb(); loadContent(id); }

function goBack() { if (historyStack.length) { currentFolderId = historyStack.pop(); currentFolderPath.pop(); updateBreadcrumb(); loadContent(currentFolderId); } }

function updateBreadcrumb() {
    const bc = document.getElementById('breadcrumb');
    bc.innerHTML = currentFolderPath.map((p, i) => i === currentFolderPath.length-1 ? `<span class="current">${p.name}</span>` : `<a onclick="navigateTo(${i})">${p.name}</a> <span>/</span>`).join('');
}

function navigateTo(i) { currentFolderPath = currentFolderPath.slice(0, i+1); currentFolderId = currentFolderPath[i].id; historyStack = []; loadContent(currentFolderId); }

// ==================== ОПЕРАЦИИ ====================
async function uploadFiles(files) {
    const token = getToken();
    for (let f of files) {
        const fd = new FormData();
        fd.append('file', f);
        if (currentFolderId && !isLibrary) fd.append('folder_id', currentFolderId);
        const url = isLibrary ? '/api/library/upload' : '/api/files/upload';
        try {
            const response = await fetch(url, { method: 'POST', headers: { 'Authorization': `Bearer ${token}` }, body: fd });
            if (!response.ok) { const error = await response.json(); alert(`Ошибка загрузки ${f.name}: ${error.error || 'Неизвестная ошибка'}`); }
        } catch(e) { console.error('Upload error:', e); alert(`Ошибка загрузки ${f.name}`); }
    }
    loadContent(currentFolderId);
}

async function deleteItem(id, isFolder) {
    if (!confirm(`Удалить ${isFolder ? 'папку' : 'файл'}?`)) return;
    const token = getToken();
    const url = isFolder ? `/api/folders/${id}` : `/api/files/${id}`;
    const response = await fetch(url, { method: 'DELETE', headers: { 'Authorization': `Bearer ${token}` } });
    if (response.ok) loadContent(currentFolderId);
}

async function downloadFile(fileId) {
    const token = getToken();
    window.location.href = `/api/files/${fileId}/download?token=${encodeURIComponent(token)}`;
}

function showCreateFolderModal() { document.getElementById('folderModal').style.display = 'flex'; document.getElementById('folderName').focus(); }
function closeFolderModal() { document.getElementById('folderModal').style.display = 'none'; }

async function createFolder() {
    const name = document.getElementById('folderName').value.trim();
    if (!name) return;
    const token = getToken();
    const data = { name };
    if (currentFolderId && !isLibrary) data.parent_folder_id = currentFolderId;
    const url = isLibrary ? '/api/library/folders' : '/api/folders';
    await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }, body: JSON.stringify(data) });
    closeFolderModal();
    loadContent(currentFolderId);
}

let renameId = null, renameIsFolder = false;

function startRename(id, isFolder, name) { renameId = id; renameIsFolder = isFolder; document.getElementById('renameInput').value = name; document.getElementById('renameModal').style.display = 'flex'; }
function closeRenameModal() { document.getElementById('renameModal').style.display = 'none'; renameId = null; }

async function renameSubmit() {
    const newName = document.getElementById('renameInput').value.trim();
    if (!newName || !renameId) return;
    const token = getToken();
    const url = renameIsFolder ? `/api/folders/${renameId}` : `/api/files/${renameId}`;
    await fetch(url, { method: 'PUT', headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }, body: JSON.stringify({ name: newName }) });
    closeRenameModal();
    loadContent(currentFolderId);
}

async function updateStorageUsed() {
    const token = getToken();
    try {
        const res = await fetch('/api/storage/stats', { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) {
            const stats = await res.json();
            const totalSize = stats.total_size || 0;
            const fileCount = stats.file_count || 0;
            const folderCount = stats.folder_count || 0;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            let size = totalSize, idx = 0;
            while (size >= 1024 && idx < sizes.length - 1) { size /= 1024; idx++; }
            document.getElementById('storageUsed').textContent = size.toFixed(2) + ' ' + sizes[idx];
            document.getElementById('storageDetails').textContent = `${fileCount} файлов • ${folderCount} папок`;
            const maxStorage = 100 * 1024 * 1024;
            const percent = Math.min((totalSize / maxStorage) * 100, 100);
            document.getElementById('storageBar').style.width = percent + '%';
        }
    } catch(e) { console.error(e); }
}

// ==================== ПЕРЕНОС В БИБЛИОТЕКУ ====================
async function moveToLibrary(itemId, itemName, isFolder) {
    if (!confirm(`Переместить "${itemName}" в библиотеку?`)) return;
    try {
        const response = await fetch(`/api/files/${itemId}/move-to-library`, {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${getToken()}`, 'Content-Type': 'application/json' },
            body: JSON.stringify({ library_parent_id: null }),
        });
        if (response.ok) {
            showToast(`✅ "${itemName}" перемещён в библиотеку`);
            loadContent(currentFolderId);
        } else {
            const data = await response.json();
            showToast('❌ ' + (data.error || 'Ошибка'));
        }
    } catch (error) { showToast('❌ Ошибка сети'); }
}

// ==================== ИНИЦИАЛИЗАЦИЯ ====================
document.addEventListener('DOMContentLoaded', async () => {
    const urlToken = new URLSearchParams(location.search).get('token');
    if (urlToken) { localStorage.setItem('token', urlToken); history.replaceState({}, '', window.location.pathname); }
    if (!getToken()) { location.href = '/login'; return; }
    await loadUserInfo();
    await checkIfUserIsAdmin();
    loadRootFiles();
    document.querySelectorAll('.modal').forEach(m => m.addEventListener('click', e => e.target === m && (m.style.display = 'none')));
    const backZone = document.getElementById('backNavZone');
    backZone.addEventListener('dragover', onBackZoneDragOver);
    backZone.addEventListener('dragleave', onBackZoneDragLeave);
    backZone.addEventListener('drop', onBackZoneDrop);
    backZone.addEventListener('click', (e) => {
        e.stopPropagation();
        if (moveModeActive) { if (currentFolderPath.length > 1) { goBack(); showToast(`Объект в руке, выберите папку`); } }
        else if (currentFolderPath.length > 1) { goBack(); }
    });
    document.querySelectorAll('.bubble-card, .file-card').forEach(el => {
        el.addEventListener('touchmove', (e) => { if (moveModeActive) e.preventDefault(); }, { passive: false });
    });
});