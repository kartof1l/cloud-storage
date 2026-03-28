// ==================== ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ ====================
let currentParentId = null;
let currentFolderPath = [{ id: null, name: 'Библиотека' }];
let currentUser = null;
let isAdmin = false;
let renameId = null;
let renameIsFolder = false;
let pendingMoveItem = null;
let longPressTimer = null;
let currentLongPressCard = null;
let touchMoveTarget = null;
let moveModeActive = false;
let moveModeItem = null;

// ==================== ПОЛЬЗОВАТЕЛЬ ====================
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

async function checkAdmin() {
    const token = getToken();
    try {
        const response = await fetch('/api/library/admins', { headers: { 'Authorization': `Bearer ${token}` } });
        if (response.ok) {
            const admins = await response.json();
            isAdmin = admins.some(a => a.email === currentUser?.email);
            document.getElementById('adminButtons').style.display = isAdmin ? 'flex' : 'none';
        }
    } catch (error) { console.error('Ошибка проверки прав:', error); }
}

// ==================== ПЕРЕМЕЩЕНИЕ ====================
async function executeMove(itemId, itemType, targetFolderId) {
    const token = getToken();
    const url = `/api/library/items/${itemId}/move`;
    try {
        const response = await fetch(url, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ parent_id: targetFolderId })
        });
        if (response.ok) {
            deactivateMoveMode();
            loadContent(currentParentId);
            return true;
        } else {
            const error = await response.json();
            alert('Ошибка перемещения: ' + (error.error || 'Неизвестная ошибка'));
            return false;
        }
    } catch (error) { 
        console.error('Move error:', error); 
        alert('Ошибка соединения');
        return false;
    }
}

function activateMoveMode(id, type, name) {
    if (moveModeActive) deactivateMoveMode();
    moveModeActive = true;
    moveModeItem = { id, type, name };
    
    document.querySelectorAll('.bubble-card, .file-card').forEach(card => {
        if (card.dataset.id !== id) {
            card.classList.add('move-target');
        }
    });
    
    showToast(`Перемещение: ${name} — нажмите на папку`, 1500);
}

function deactivateMoveMode() {
    moveModeActive = false;
    moveModeItem = null;
    document.querySelectorAll('.move-target').forEach(el => el.classList.remove('move-target'));
}

function handleItemClickForMove(id, name) {
    if (!moveModeActive) return false;
    if (moveModeItem.id === id) {
        showToast('Нельзя переместить в самого себя');
        return false;
    }
    executeMove(moveModeItem.id, moveModeItem.type, id);
    return true;
}

// ==================== DRAG & DROP ДЛЯ БИБЛИОТЕКИ ====================
function startDrag(e, id, type, name) {
    if (!isAdmin) return;
    pendingMoveItem = { id, type, name };
    e.dataTransfer.setData('text/plain', JSON.stringify({ id, type, name }));
    e.dataTransfer.effectAllowed = 'move';
    const element = e.target.closest('.bubble-card, .file-card');
    if (element) element.classList.add('dragging');
}

function endDrag(e) {
    document.querySelectorAll('.dragging').forEach(el => el.classList.remove('dragging'));
    document.querySelectorAll('.drag-over').forEach(el => el.classList.remove('drag-over'));
}

function dragOver(e, targetId, targetType) {
    e.preventDefault();
    if (!pendingMoveItem || !isAdmin) return;
    if (pendingMoveItem.id === targetId) return;
    if (pendingMoveItem.type === 'folder' && targetType === 'file') return;
    e.dataTransfer.dropEffect = 'move';
}

function dragEnter(e, targetId) {
    if (!pendingMoveItem || !isAdmin) return;
    if (pendingMoveItem.id === targetId) return;
    const target = document.querySelector(`[data-id="${targetId}"]`);
    if (target) target.classList.add('drag-over');
}

function dragLeave(e, targetId) {
    const target = document.querySelector(`[data-id="${targetId}"]`);
    if (target) target.classList.remove('drag-over');
}

function dropOnFolder(e, targetFolderId) {
    e.preventDefault();
    if (!pendingMoveItem || !isAdmin) return;
    if (pendingMoveItem.id === targetFolderId) { endDrag(e); return; }
    executeMove(pendingMoveItem.id, pendingMoveItem.type, targetFolderId);
    endDrag(e);
}

async function showFolderPickerForMove(itemId, itemType, itemName) {
    if (!isAdmin) { alert('Только администраторы могут перемещать'); return; }
    pendingMoveItem = { id: itemId, type: itemType, name: itemName };
    await loadFolderPickerTree();
    document.getElementById('folderPicker').style.display = 'flex';
}

async function loadFolderPickerTree() {
    const token = getToken();
    const container = document.getElementById('folderPickerList');
    container.innerHTML = '<div style="text-align:center;padding:20px;">Загрузка...</div>';
    try {
        const res = await fetch('/api/library/items', { headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) {
            const items = await res.json();
            const folders = (items || []).filter(f => f.is_folder);
            if (folders.length === 0) {
                container.innerHTML = '<div style="text-align:center;padding:20px;">Нет доступных папок</div>';
                return;
            }
            container.innerHTML = `<div class="folder-picker-item" onclick="executeMoveToRoot()"><div class="icon">📁</div><div class="name">📁 Корневая папка библиотеки</div><div class="arrow">→</div></div>`;
            folders.forEach(folder => {
                if (folder.id === pendingMoveItem?.id) return;
                container.innerHTML += `<div class="folder-picker-item" onclick="executeMove('${folder.id}', '${escapeHtml(folder.name)}')"><div class="icon">📁</div><div class="name">${escapeHtml(folder.name)}</div><div class="arrow">→</div></div>`;
            });
        }
    } catch(e) { container.innerHTML = '<div style="text-align:center;padding:20px;">Ошибка загрузки</div>'; console.error(e); }
}

function executeMoveToRoot() { executeMove(pendingMoveItem.id, pendingMoveItem.type, null); }

function closeFolderPicker() { 
    document.getElementById('folderPicker').style.display = 'none'; 
    pendingMoveItem = null; 
}

// ==================== МОБИЛЬНЫЕ ЖЕСТЫ ====================
function onTouchStart(e, id, type, name) {
    const card = e.target.closest('.bubble-card, .file-card');
    if (!card) return;
    
    if (e.target.closest('.back-action-btn')) return;
    
    e.preventDefault();
    currentLongPressCard = { id, type, name, card };
    longPressTimer = setTimeout(() => {
        if (currentLongPressCard) {
            currentLongPressCard.card.classList.add('tapped');
            activateMoveMode(id, type, name);
            showToast(`Удерживайте и перетащите на папку`);
        }
    }, 500);
}

function onTouchMove(e) {
    if (!moveModeActive || !moveModeItem) return;
    e.preventDefault();
    
    const touch = e.touches[0];
    const elementUnderFinger = document.elementFromPoint(touch.clientX, touch.clientY);
    const targetCard = elementUnderFinger?.closest('.bubble-card, .file-card');
    
    document.querySelectorAll('.bubble-card, .file-card').forEach(card => card.classList.remove('drag-over'));
    
    if (targetCard && targetCard.dataset.id !== moveModeItem.id) {
        targetCard.classList.add('drag-over');
        touchMoveTarget = targetCard.dataset.id;
    } else {
        touchMoveTarget = null;
    }
}

function onTouchEnd(e) {
    if (longPressTimer) {
        clearTimeout(longPressTimer);
        longPressTimer = null;
    }
    
    if (moveModeActive && moveModeItem) {
        if (touchMoveTarget && touchMoveTarget !== moveModeItem.id) {
            executeMove(moveModeItem.id, moveModeItem.type, touchMoveTarget);
        } else {
            deactivateMoveMode();
        }
    }
    
    if (currentLongPressCard) {
        setTimeout(() => {
            if (currentLongPressCard.card) currentLongPressCard.card.classList.remove('tapped');
        }, 100);
        currentLongPressCard = null;
    }
    touchMoveTarget = null;
}

// ==================== ЗАГРУЗКА КОНТЕНТА ====================
async function loadContent(parentId = null) {
    const token = getToken();
    const url = parentId ? `/api/library/items?parent_id=${parentId}` : '/api/library/items';
    try {
        const response = await fetch(url, { headers: { 'Authorization': `Bearer ${token}` } });
        if (response.ok) {
            const items = await response.json();
            displayContent(items || []);
            updateBreadcrumb();
        } else if (response.status === 401) {
            window.location.href = '/login';
        }
    } catch (error) {
        console.error('Ошибка загрузки:', error);
        displayContent([]);
    }
}

function displayContent(items) {
    const folders = items.filter(f => f.is_folder);
    const files = items.filter(f => !f.is_folder);
    const bubblesSection = document.getElementById('bubblesSection');
    const filesSection = document.getElementById('filesSection');
    const emptyState = document.getElementById('emptyState');

    if (items.length === 0) {
        bubblesSection.style.display = 'none';
        filesSection.style.display = 'none';
        emptyState.style.display = 'block';
        return;
    }
    emptyState.style.display = 'none';
    if (folders.length > 0) {
        bubblesSection.style.display = 'block';
        displayBubbles(folders);
    } else bubblesSection.style.display = 'none';
    if (files.length > 0) {
        filesSection.style.display = 'block';
        displayFiles(files);
    } else filesSection.style.display = 'none';
}

function displayBubbles(folders) {
    const container = document.getElementById('bubblesContainer');
    container.innerHTML = folders.map((f, i) => `
        <div class="bubble-card" data-id="${f.id}" data-name="${escapeHtml(f.name)}"
             draggable="${isAdmin}"
             ondragstart="${isAdmin ? `startDrag(event, '${f.id}', 'folder', '${escapeHtml(f.name)}')` : ''}"
             ondragend="endDrag(event)"
             ondrop="dropOnFolder(event, '${f.id}')"
             ondragover="dragOver(event, '${f.id}', 'folder')"
             ondragenter="dragEnter(event, '${f.id}')"
             ondragleave="dragLeave(event, '${f.id}')">
            <div class="card-inner">
                <div class="card-front">
                    <div class="bubble-icon" style="background: ${getGradient(i)}">📁</div>
                    <div class="bubble-name">${escapeHtml(f.name)}</div>
                    <div class="bubble-size">${formatFileSize(f.size || 0)}</div>
                </div>
                <div class="card-back">
                    <div class="back-actions">
                        <div class="back-action-btn" onclick="event.stopPropagation(); openFolder('${f.id}', '${escapeHtml(f.name)}')">📂 <span>Открыть</span></div>
                        ${isAdmin ? `
                            <div class="back-action-btn" onclick="event.stopPropagation(); startRename('${f.id}', true, '${escapeHtml(f.name)}')">✏️ <span>Ред.</span></div>
                            <div class="back-action-btn" onclick="event.stopPropagation(); deleteItem('${f.id}', true)">🗑️ <span>Удалить</span></div>
                            <div class="back-action-btn" onclick="event.stopPropagation(); showFolderPickerForMove('${f.id}', 'folder', '${escapeHtml(f.name)}')">🔄 <span>Перем.</span></div>
                        ` : ''}
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
            if (moveModeActive) {
                handleItemClickForMove(folderId, folderName);
            } else {
                openFolder(folderId, folderName);
            }
        });
        
        if (isTouchDevice) {
            card.addEventListener('touchstart', (e) => onTouchStart(e, folderId, 'folder', folderName), { passive: false });
            card.addEventListener('touchmove', onTouchMove, { passive: false });
            card.addEventListener('touchend', onTouchEnd);
        }
    });
}

function displayFiles(files) {
    const container = document.getElementById('fileGrid');
    const token = getToken();
    container.innerHTML = files.map(file => {
        const isImage = file.mime_type?.startsWith('image/');
        const previewUrl = `/api/library/items/${file.id}/download?token=${encodeURIComponent(token)}`;
        return `
            <div class="file-card" data-id="${file.id}" data-name="${escapeHtml(file.name)}"
                 draggable="${isAdmin}"
                 ondragstart="${isAdmin ? `startDrag(event, '${file.id}', 'file', '${escapeHtml(file.name)}')` : ''}"
                 ondragend="endDrag(event)"
                 ondrop="dropOnFolder(event, '${currentParentId || 'null'}')"
                 ondragover="dragOver(event, '${currentParentId}', 'folder')">
                <div class="card-inner">
                    <div class="card-front">
                        <div class="file-preview">
                            ${isImage ? `<img src="${previewUrl}" onerror="this.parentElement.innerHTML='📄'">` : `<span>${getFileIcon(file.mime_type)}</span>`}
                        </div>
                        <div class="file-name">${escapeHtml(file.name)}</div>
                        <div class="file-meta">${formatFileSize(file.size)}</div>
                    </div>
                    <div class="card-back">
                        <div class="back-actions">
                            <div class="back-action-btn" onclick="event.stopPropagation(); downloadFile('${file.id}')">⬇️ <span>Скачать</span></div>
                            ${isAdmin ? `
                                <div class="back-action-btn" onclick="event.stopPropagation(); startRename('${file.id}', false, '${escapeHtml(file.name)}')">✏️ <span>Ред.</span></div>
                                <div class="back-action-btn" onclick="event.stopPropagation(); deleteItem('${file.id}', false)">🗑️ <span>Удалить</span></div>
                                <div class="back-action-btn" onclick="event.stopPropagation(); showFolderPickerForMove('${file.id}', 'file', '${escapeHtml(file.name)}')">🔄 <span>Перем.</span></div>
                            ` : ''}
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
            if (moveModeActive) {
                handleItemClickForMove(fileId, fileName);
            } else {
                downloadFile(fileId);
            }
        });
        
        if (isTouchDevice) {
            card.addEventListener('touchstart', (e) => onTouchStart(e, fileId, 'file', fileName), { passive: false });
            card.addEventListener('touchmove', onTouchMove, { passive: false });
            card.addEventListener('touchend', onTouchEnd);
        }
    });
}

// ==================== НАВИГАЦИЯ ====================
function loadRoot() { 
    currentParentId = null; 
    currentFolderPath = [{ id: null, name: 'Библиотека' }]; 
    updateBreadcrumb(); 
    loadContent(null); 
}

function openFolder(id, name) { 
    currentParentId = id; 
    currentFolderPath.push({ id, name }); 
    updateBreadcrumb(); 
    loadContent(id); 
}

function goBack() { 
    if (currentFolderPath.length > 1) { 
        currentFolderPath.pop(); 
        currentParentId = currentFolderPath[currentFolderPath.length - 1].id; 
        updateBreadcrumb(); 
        loadContent(currentParentId); 
    } 
}

function updateBreadcrumb() { 
    const bc = document.getElementById('breadcrumb'); 
    bc.innerHTML = currentFolderPath.map((p, i) => i === currentFolderPath.length-1 ? `<span class="current">${p.name}</span>` : `<a onclick="navigateTo(${i})">${p.name}</a> <span>/</span>`).join(''); 
}

function navigateTo(index) { 
    currentFolderPath = currentFolderPath.slice(0, index+1); 
    currentParentId = currentFolderPath[index].id; 
    updateBreadcrumb(); 
    loadContent(currentParentId); 
}

// ==================== ОПЕРАЦИИ ====================
async function uploadFiles(files) {
    if (!isAdmin) { alert('Только администраторы могут загружать файлы'); return; }
    const token = getToken();
    for (let file of files) {
        const formData = new FormData();
        formData.append('file', file);
        if (currentParentId) formData.append('parent_id', currentParentId);
        try {
            const response = await fetch('/api/library/upload', { 
                method: 'POST', 
                headers: { 'Authorization': `Bearer ${token}` }, 
                body: formData 
            });
            if (!response.ok) {
                const error = await response.json();
                alert(`Ошибка загрузки ${file.name}: ${error.error || 'Неизвестная ошибка'}`);
            }
        } catch(e) {
            console.error('Upload error:', e);
            alert(`Ошибка загрузки ${file.name}`);
        }
    }
    await loadContent(currentParentId);
    await updateStorageUsed();
}

async function downloadFile(fileId) {
    const token = getToken();
    window.location.href = `/api/library/items/${fileId}/download?token=${encodeURIComponent(token)}`;
}

async function deleteItem(id, isFolder) {
    if (!isAdmin) { alert('Только администраторы могут удалять'); return; }
    if (!confirm(`Удалить ${isFolder ? 'папку' : 'файл'}?`)) return;
    const token = getToken();
    const response = await fetch(`/api/library/items/${id}`, { method: 'DELETE', headers: { 'Authorization': `Bearer ${token}` } });
    if (response.ok) {
        await loadContent(currentParentId);
        await updateStorageUsed();
    }
}

function showCreateFolderModal() { 
    if (!isAdmin) { alert('Только администраторы могут создавать папки'); return; } 
    document.getElementById('folderModal').style.display = 'flex'; 
    document.getElementById('folderName').focus(); 
}

function closeFolderModal() { 
    document.getElementById('folderModal').style.display = 'none'; 
}

async function createFolder() {
    const name = document.getElementById('folderName').value.trim();
    if (!name) return alert('Введите название');
    const token = getToken();
    const data = { name };
    if (currentParentId) data.parent_id = currentParentId;
    const response = await fetch('/api/library/folders', { 
        method: 'POST', 
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }, 
        body: JSON.stringify(data) 
    });
    if (response.ok) { 
        closeFolderModal(); 
        await loadContent(currentParentId);
        await updateStorageUsed();
    }
}

function startRename(id, isFolder, name) { 
    renameId = id; 
    renameIsFolder = isFolder; 
    document.getElementById('renameInput').value = name; 
    document.getElementById('renameModal').style.display = 'flex'; 
}

function closeRenameModal() { 
    document.getElementById('renameModal').style.display = 'none'; 
    renameId = null; 
}

async function renameSubmit() {
    const newName = document.getElementById('renameInput').value.trim();
    if (!newName || !renameId) return;
    const token = getToken();
    const response = await fetch(`/api/library/items/${renameId}`, { 
        method: 'PUT', 
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }, 
        body: JSON.stringify({ name: newName }) 
    });
    if (response.ok) { 
        closeRenameModal(); 
        await loadContent(currentParentId);
        await updateStorageUsed();
    }
}

async function updateStorageUsed() {
    const token = getToken();
    if (!token) return;
    
    try {
        const res = await fetch('/api/library/stats', { 
            headers: { 'Authorization': `Bearer ${token}` } 
        });
        
        if (res.ok) {
            const stats = await res.json();
            const totalSize = stats.total_size || 0;
            const fileCount = stats.file_count || 0;
            const folderCount = stats.folder_count || 0;
            
            const sizes = ['B', 'KB', 'MB', 'GB'];
            let size = totalSize, idx = 0;
            while (size >= 1024 && idx < sizes.length - 1) { 
                size /= 1024; 
                idx++; 
            }
            
            document.getElementById('storageUsed').textContent = size.toFixed(2) + ' ' + sizes[idx];
            document.getElementById('storageDetails').textContent = `${fileCount} файлов • ${folderCount} папок`;
            
            const maxStorage = 1024 * 1024 * 1024;
            const percent = Math.min((totalSize / maxStorage) * 100, 100);
            document.getElementById('storageBar').style.width = percent + '%';
        } else {
            console.error('Failed to get library stats');
        }
    } catch(e) { 
        console.error('Error updating library stats:', e);
    }
}

// ==================== ИНИЦИАЛИЗАЦИЯ ====================
document.addEventListener('DOMContentLoaded', async () => {
    if (!getToken()) { window.location.href = '/login'; return; }
    await loadUserInfo();
    await checkAdmin();
    loadRoot();
    updateStorageUsed();
    
    document.querySelectorAll('.modal').forEach(m => m.addEventListener('click', e => e.target === m && (m.style.display = 'none')));
    document.getElementById('folderPicker').addEventListener('click', e => { if (e.target === document.getElementById('folderPicker')) closeFolderPicker(); });
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape') document.querySelectorAll('.modal').forEach(m => m.style.display = 'none'); });
    
    document.querySelectorAll('.bubble-card, .file-card').forEach(el => {
        el.addEventListener('touchmove', (e) => {
            if (moveModeActive) e.preventDefault();
        }, { passive: false });
    });
});