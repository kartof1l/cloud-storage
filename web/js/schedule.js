// ==================== ТАСК-МЕНЕДЖЕР ====================
let currentWeekOffset = 0;
let tasksList = [];
let editingTaskId = null;
let currentView = 'calendar';

function isUserAdmin() {
    return window.isCurrentUserAdmin === true;
}

async function loadTasks() {
    const token = getToken();
    if (!token) return;
    
    try {
        const res = await fetch('/api/schedule/tasks', {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (res.ok) {
            tasksList = await res.json();
            renderCurrentView();
            updateStats();
            const quickAddBlock = document.getElementById('quickAddBlock');
            if (quickAddBlock) quickAddBlock.style.display = isUserAdmin() ? 'block' : 'none';
        } else {
            console.error('Failed to load tasks:', res.status);
        }
    } catch(e) {
        console.error('Error loading tasks:', e);
    }
}

function updateStats() {
    const total = tasksList.length;
    const completed = tasksList.filter(t => t.completed).length;
    const pending = total - completed;
    const highPriority = tasksList.filter(t => t.priority === 'high' && !t.completed).length;
    const overdue = tasksList.filter(t => !t.completed && t.due_date && t.due_date < formatDate(new Date())).length;
    
    const statsContainer = document.getElementById('tasksStats');
    if (statsContainer) {
        statsContainer.innerHTML = `
            <div class="stat-badge">📊 Всего: <span class="count">${total}</span></div>
            <div class="stat-badge">✅ Выполнено: <span class="count">${completed}</span></div>
            <div class="stat-badge">⏳ Осталось: <span class="count">${pending}</span></div>
            <div class="stat-badge">🔴 Важных: <span class="count">${highPriority}</span></div>
            ${overdue > 0 ? `<div class="stat-badge" style="background: rgba(255,77,109,0.2);">⚠️ Просрочено: <span class="count" style="color:var(--danger);">${overdue}</span></div>` : ''}
        `;
    }
}

function switchView(view) {
    currentView = view;
    document.querySelectorAll('.view-btn').forEach(btn => btn.classList.remove('active'));
    const activeBtn = document.querySelector(`.view-btn[data-view="${view}"]`);
    if (activeBtn) activeBtn.classList.add('active');
    const calendarView = document.getElementById('calendarView');
    const listView = document.getElementById('listView');
    if (calendarView) calendarView.classList.toggle('active', view === 'calendar');
    if (listView) listView.classList.toggle('active', view === 'list');
    if (view === 'list') renderListView();
    else renderCalendarView();
}

function renderCurrentView() {
    if (currentView === 'calendar') renderCalendarView();
    else renderListView();
    updateStats();
}

function getWeekDates(offset = 0) {
    const today = new Date();
    const currentDay = today.getDay();
    const diffToMonday = currentDay === 0 ? -6 : 1 - currentDay;
    const monday = new Date(today);
    monday.setDate(today.getDate() + diffToMonday + (offset * 7));
    monday.setHours(0, 0, 0, 0);
    const weekDates = [];
    for (let i = 0; i < 7; i++) {
        const date = new Date(monday);
        date.setDate(monday.getDate() + i);
        weekDates.push(date);
    }
    return weekDates;
}

function formatDate(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
}

function formatDisplayDate(date) {
    const months = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек'];
    return `${date.getDate()} ${months[date.getMonth()]}`;
}

function formatDisplayDateStr(dateStr) {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    const months = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек'];
    return `${date.getDate()} ${months[date.getMonth()]}`;
}

function getDayName(date) {
    const days = ['ПН', 'ВТ', 'СР', 'ЧТ', 'ПТ', 'СБ', 'ВС'];
    return days[date.getDay() === 0 ? 6 : date.getDay() - 1];
}

function getPriorityIcon(priority) {
    const icons = { high: '🔴', medium: '🟡', low: '🟢' };
    return icons[priority] || '⚪';
}

function getPriorityName(priority) {
    const names = { high: 'Высокий', medium: 'Средний', low: 'Низкий' };
    return names[priority] || 'Обычный';
}

function isTaskOverdue(task) {
    if (task.completed) return false;
    if (!task.due_date) return false;
    const today = formatDate(new Date());
    return task.due_date < today;
}

function renderCalendarView() {
    const weekDates = getWeekDates(currentWeekOffset);
    const container = document.getElementById('scheduleGrid');
    if (!container) return;
    
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const todayStr = formatDate(today);
    const startWeek = weekDates[0];
    const endWeek = weekDates[6];
    const weekLabel = `${formatDisplayDate(startWeek)} - ${formatDisplayDate(endWeek)}`;
    const weekLabelEl = document.getElementById('currentWeekLabel');
    if (weekLabelEl) weekLabelEl.textContent = weekLabel;
    
    container.innerHTML = weekDates.map((date) => {
        const dateStr = formatDate(date);
        const isToday = dateStr === todayStr;
        const dayTasks = tasksList.filter(t => t.due_date === dateStr);
        dayTasks.sort((a, b) => {
            if (a.completed !== b.completed) return a.completed ? 1 : -1;
            const priorityOrder = { high: 0, medium: 1, low: 2 };
            return (priorityOrder[a.priority] || 3) - (priorityOrder[b.priority] || 3);
        });
        
        return `
            <div class="schedule-day ${isToday ? 'day-today' : ''}">
                <div class="day-header">
                    <div class="day-name">${getDayName(date)}</div>
                    <div class="day-date">${formatDisplayDate(date)}</div>
                </div>
                <div class="day-events">
                    ${isUserAdmin() ? `<button class="btn btn-primary add-task-btn" onclick="showTaskModal('${dateStr}')">+ Добавить задачу</button>` : ''}
                    ${dayTasks.length === 0 ? '<div class="empty-events">📭 Нет задач</div>' :
                        dayTasks.map(task => {
                            const isOverdue = isTaskOverdue(task);
                            return `
                                <div class="task-card ${task.completed ? 'completed-task' : ''} ${isOverdue ? 'overdue-task' : ''}">
                                    <div class="task-card-time">🕐 ${task.due_time || 'Любое время'}${isOverdue ? ' <span class="overdue-badge">⚠️ Просрочено</span>' : ''}${task.completed ? ' <span class="completed-badge">✅ Выполнено</span>' : ''}</div>
                                    <div class="task-card-title ${task.completed ? 'completed' : ''}">${escapeHtml(task.title)}</div>
                                    ${task.description ? `<div class="task-card-desc">${escapeHtml(task.description)}</div>` : ''}
                                    <div class="task-card-footer">
                                        <span class="task-card-priority ${task.priority}">${getPriorityIcon(task.priority)} ${getPriorityName(task.priority)}</span>
                                        <div class="task-card-actions">
                                            ${!task.completed ? `<button class="task-action-btn complete-btn" onclick="toggleTaskComplete('${task.id}')">✅</button>` : `<button class="task-action-btn revert-btn" onclick="toggleTaskComplete('${task.id}')">↩️</button>`}
                                            ${isUserAdmin() ? `<button class="task-action-btn edit-btn" onclick="editTask('${task.id}')">✏️</button><button class="task-action-btn delete-btn" onclick="deleteTask('${task.id}')">🗑️</button>` : ''}
                                        </div>
                                    </div>
                                </div>
                            `;
                        }).join('')
                    }
                </div>
            </div>
        `;
    }).join('');
}

function renderListView() {
    const container = document.getElementById('tasksListContainer');
    if (!container) return;
    
    const groups = {
        overdue: tasksList.filter(t => !t.completed && isTaskOverdue(t)),
        pending: tasksList.filter(t => !t.completed && !isTaskOverdue(t)),
        completed: tasksList.filter(t => t.completed)
    };
    
    groups.overdue.sort((a, b) => (a.due_date || '').localeCompare(b.due_date || ''));
    groups.pending.sort((a, b) => {
        const priorityOrder = { high: 0, medium: 1, low: 2 };
        if (a.priority !== b.priority) return (priorityOrder[a.priority] || 3) - (priorityOrder[b.priority] || 3);
        if (a.due_date !== b.due_date) return (a.due_date || '9999-99-99').localeCompare(b.due_date || '9999-99-99');
        return (a.due_time || '').localeCompare(b.due_time || '');
    });
    groups.completed.sort((a, b) => (b.updated_at || '').localeCompare(a.updated_at || ''));
    
    container.innerHTML = `
        ${groups.overdue.length > 0 ? `<div class="task-group"><div class="task-group-title overdue-title">⚠️ Просроченные (${groups.overdue.length})</div>${groups.overdue.map(task => renderTaskItem(task, true)).join('')}</div>` : ''}
        <div class="task-group"><div class="task-group-title">⏳ Активные задачи (${groups.pending.length})</div>${groups.pending.length === 0 && groups.overdue.length === 0 ? '<div class="empty-events">✨ Все задачи выполнены!</div>' : ''}${groups.pending.map(task => renderTaskItem(task, false)).join('')}</div>
        ${groups.completed.length > 0 ? `<div class="task-group"><div class="task-group-title">✅ Выполненные (${groups.completed.length})</div>${groups.completed.map(task => renderTaskItem(task, false)).join('')}</div>` : ''}
    `;
}

function renderTaskItem(task, isOverdue = false) {
    return `
        <div class="task-item ${task.completed ? 'completed' : ''} ${isOverdue ? 'overdue' : ''}">
            <div class="task-checkbox ${task.completed ? 'completed' : ''}" onclick="toggleTaskComplete('${task.id}')"></div>
            <div class="task-content">
                <div class="task-title ${task.completed ? 'completed' : ''}">${escapeHtml(task.title)}</div>
                ${task.description ? `<div class="task-desc">${escapeHtml(task.description)}</div>` : ''}
                <div class="task-meta">
                    ${task.due_date ? `<span class="task-date">📅 ${formatDisplayDateStr(task.due_date)}${isOverdue ? ' (просрочено)' : ''}</span>` : ''}
                    ${task.due_time ? `<span class="task-time">⏰ ${task.due_time}</span>` : ''}
                    <span class="task-priority ${task.priority}">${getPriorityIcon(task.priority)} ${getPriorityName(task.priority)}</span>
                </div>
            </div>
            <div class="task-actions">
                ${!task.completed ? `<button class="task-action-btn complete-btn" onclick="toggleTaskComplete('${task.id}')">✅ Выполнить</button>` : `<button class="task-action-btn revert-btn" onclick="toggleTaskComplete('${task.id}')">↩️ Вернуть</button>`}
                ${isUserAdmin() ? `<button class="task-action-btn edit-btn" onclick="editTask('${task.id}')">✏️</button><button class="task-action-btn delete-btn" onclick="deleteTask('${task.id}')">🗑️</button>` : ''}
            </div>
        </div>
    `;
}

function showTaskModal(prefilledDate = '') {
    editingTaskId = null;
    document.getElementById('taskModalTitle').textContent = '➕ Новая задача';
    document.getElementById('taskTitle').value = '';
    document.getElementById('taskDescription').value = '';
    document.getElementById('taskDueDate').value = prefilledDate || formatDate(new Date());
    document.getElementById('taskDueTime').value = '';
    document.getElementById('taskPriority').value = 'medium';
    document.getElementById('taskModal').style.display = 'flex';
}

async function editTask(taskId) {
    const task = tasksList.find(t => t.id === taskId);
    if (!task) return;
    editingTaskId = taskId;
    document.getElementById('taskModalTitle').textContent = '✏️ Редактировать задачу';
    document.getElementById('taskTitle').value = task.title;
    document.getElementById('taskDescription').value = task.description || '';
    document.getElementById('taskDueDate').value = task.due_date || '';
    document.getElementById('taskDueTime').value = task.due_time || '';
    document.getElementById('taskPriority').value = task.priority || 'medium';
    document.getElementById('taskModal').style.display = 'flex';
}

async function saveTask() {
    const token = getToken();
    const title = document.getElementById('taskTitle').value.trim();
    const description = document.getElementById('taskDescription').value.trim();
    const dueDate = document.getElementById('taskDueDate').value;
    const dueTime = document.getElementById('taskDueTime').value;
    const priority = document.getElementById('taskPriority').value;
    if (!title) { alert('Введите название задачи'); return; }
    const data = { title, description, due_date: dueDate, due_time: dueTime, priority };
    try {
        let res;
        if (editingTaskId) {
            res = await fetch(`/api/schedule/tasks/${editingTaskId}`, { method: 'PUT', headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }, body: JSON.stringify(data) });
        } else {
            res = await fetch('/api/schedule/tasks', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }, body: JSON.stringify(data) });
        }
        if (res.ok) { closeTaskModal(); loadTasks(); showToast(editingTaskId ? 'Задача обновлена' : 'Задача добавлена'); }
        else { const error = await res.json(); alert('Ошибка: ' + (error.error || 'Неизвестная ошибка')); }
    } catch(e) { console.error(e); alert('Ошибка сохранения'); }
}

async function toggleTaskComplete(taskId) {
    const token = getToken();
    const task = tasksList.find(t => t.id === taskId);
    if (!task) return;
    try {
        const res = await fetch(`/api/schedule/tasks/${taskId}/toggle`, { method: 'PATCH', headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }, body: JSON.stringify({ completed: !task.completed }) });
        if (res.ok) { loadTasks(); showToast(task.completed ? 'Задача возвращена' : 'Задача выполнена! 🎉'); }
    } catch(e) { console.error(e); }
}

async function deleteTask(taskId) {
    if (!confirm('Удалить задачу?')) return;
    const token = getToken();
    try {
        const res = await fetch(`/api/schedule/tasks/${taskId}`, { method: 'DELETE', headers: { 'Authorization': `Bearer ${token}` } });
        if (res.ok) { loadTasks(); showToast('Задача удалена'); }
    } catch(e) { console.error(e); }
}

function closeTaskModal() { document.getElementById('taskModal').style.display = 'none'; editingTaskId = null; }

function changeWeek(delta) { currentWeekOffset += delta; renderCalendarView(); }

function goToToday() { currentWeekOffset = 0; renderCalendarView(); setTimeout(() => { const todayCard = document.querySelector('.schedule-day.day-today'); if (todayCard) todayCard.scrollIntoView({ behavior: 'smooth', block: 'center' }); }, 100); }

function quickAddTask() {
    const title = document.getElementById('quickTaskTitle').value.trim();
    if (!title) { alert('Введите название задачи'); return; }
    const priority = document.getElementById('quickTaskPriority').value;
    const dueDate = formatDate(new Date());
    saveTaskQuick(title, priority, dueDate);
    document.getElementById('quickTaskTitle').value = '';
}

async function saveTaskQuick(title, priority, dueDate) {
    const token = getToken();
    const data = { title, priority, due_date: dueDate, description: '', due_time: '' };
    try {
        const res = await fetch('/api/schedule/tasks', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` }, body: JSON.stringify(data) });
        if (res.ok) { loadTasks(); showToast('Задача добавлена'); }
    } catch(e) { console.error(e); }
}

function switchContentTab(tab) {
    console.log('Switching to tab:', tab);
    document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
    const activeBtn = document.querySelector(`.tab-btn[data-tab="${tab}"]`);
    if (activeBtn) activeBtn.classList.add('active');
    const filesContent = document.getElementById('filesContent');
    const scheduleContent = document.getElementById('scheduleContent');
    if (filesContent) filesContent.style.display = tab === 'files' ? 'block' : 'none';
    if (scheduleContent) scheduleContent.style.display = tab === 'schedule' ? 'block' : 'none';
    if (tab === 'schedule') {
        const quickAddBlock = document.getElementById('quickAddBlock');
        if (quickAddBlock) quickAddBlock.style.display = isUserAdmin() ? 'block' : 'none';
        loadTasks();
    }
}

document.addEventListener('DOMContentLoaded', function() {
    console.log('Schedule.js loaded');
    console.log('isUserAdmin:', isUserAdmin());
});