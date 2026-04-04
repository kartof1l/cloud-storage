// ==================== ТАСК-МЕНЕДЖЕР ====================
let currentWeekOffset = 0;
let tasksList = [];
let editingTaskId = null;
let currentView = 'calendar'; // 'calendar' or 'list'

// Загрузить задачи
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
        }
    } catch(e) {
        console.error('Error loading tasks:', e);
    }
}

// Обновить статистику
function updateStats() {
    const total = tasksList.length;
    const completed = tasksList.filter(t => t.completed).length;
    const pending = total - completed;
    const highPriority = tasksList.filter(t => t.priority === 'high' && !t.completed).length;
    
    const statsHtml = `
        <div class="stat-badge">📊 Всего: <span class="count">${total}</span></div>
        <div class="stat-badge">✅ Выполнено: <span class="count">${completed}</span></div>
        <div class="stat-badge">⏳ Осталось: <span class="count">${pending}</span></div>
        <div class="stat-badge">🔴 Важных: <span class="count">${highPriority}</span></div>
    `;
    
    const statsContainer = document.getElementById('tasksStats');
    if (statsContainer) statsContainer.innerHTML = statsHtml;
}

// Переключение вида
function switchView(view) {
    currentView = view;
    
    document.querySelectorAll('.view-btn').forEach(btn => btn.classList.remove('active'));
    document.querySelector(`.view-btn[data-view="${view}"]`).classList.add('active');
    
    document.getElementById('calendarView').classList.toggle('active', view === 'calendar');
    document.getElementById('listView').classList.toggle('active', view === 'list');
    
    if (view === 'list') {
        renderListView();
    } else {
        renderCalendarView();
    }
}

function renderCurrentView() {
    if (currentView === 'calendar') {
        renderCalendarView();
    } else {
        renderListView();
    }
    updateStats();
}

// Рендер календарного вида
function renderCalendarView() {
    const weekDates = getWeekDates(currentWeekOffset);
    const container = document.getElementById('scheduleGrid');
    if (!container) return;
    
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    
    const startWeek = weekDates[0];
    const endWeek = weekDates[6];
    const weekLabel = `${formatDisplayDate(startWeek)} - ${formatDisplayDate(endWeek)}`;
    const weekLabelEl = document.getElementById('currentWeekLabel');
    if (weekLabelEl) weekLabelEl.textContent = weekLabel;
    
    container.innerHTML = weekDates.map(date => {
        const dateStr = formatDate(date);
        const isToday = dateStr === formatDate(today);
        const dayTasks = tasksList.filter(t => t.due_date === dateStr);
        
        // Сортируем задачи по времени и приоритету
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
                    ${window.isCurrentUserAdmin ? `
                        <button class="btn btn-primary" style="width:100%; margin-bottom:12px; padding:6px;" onclick="showTaskModal('${dateStr}')">
                            + Добавить
                        </button>
                    ` : ''}
                    ${dayTasks.length === 0 ? 
                        '<div class="empty-events">📭 Нет задач</div>' :
                        dayTasks.map(task => `
                            <div class="task-card" onclick="event.stopPropagation()">
                                <div class="task-card-time">
                                    <span>🕐</span> ${task.due_time || 'Любое время'}
                                </div>
                                <div class="task-card-title ${task.completed ? 'completed' : ''}">
                                    ${escapeHtml(task.title)}
                                </div>
                                ${task.description ? `<div style="font-size:10px; color:var(--text-muted); margin-bottom:6px;">${escapeHtml(task.description)}</div>` : ''}
                                <div style="display:flex; justify-content:space-between; align-items:center;">
                                    <span class="task-card-priority priority-${task.priority}">
                                        ${getPriorityIcon(task.priority)} ${getPriorityName(task.priority)}
                                    </span>
                                    ${window.isCurrentUserAdmin ? `
                                        <div style="display:flex; gap:6px;">
                                            <button class="task-action-btn" onclick="toggleTaskComplete('${task.id}')">
                                                ${task.completed ? '↩️ Вернуть' : '✅ Выполнить'}
                                            </button>
                                            <button class="task-action-btn" onclick="editTask('${task.id}')">✏️</button>
                                            <button class="task-action-btn" onclick="deleteTask('${task.id}')">🗑️</button>
                                        </div>
                                    ` : `
                                        ${!task.completed ? 
                                            `<button class="task-action-btn" onclick="markTaskComplete('${task.id}')">✅ Выполнить</button>` :
                                            ''
                                        }
                                    `}
                                </div>
                            </div>
                        `).join('')
                    }
                </div>
            </div>
        `;
    }).join('');
}

// Рендер спискового вида
function renderListView() {
    const container = document.getElementById('tasksListContainer');
    if (!container) return;
    
    const groups = {
        pending: tasksList.filter(t => !t.completed),
        completed: tasksList.filter(t => t.completed)
    };
    
    // Сортируем задачи
    groups.pending.sort((a, b) => {
        const priorityOrder = { high: 0, medium: 1, low: 2 };
        if (a.priority !== b.priority) return (priorityOrder[a.priority] || 3) - (priorityOrder[b.priority] || 3);
        if (a.due_date !== b.due_date) return (a.due_date || '9999-99-99').localeCompare(b.due_date || '9999-99-99');
        return (a.due_time || '').localeCompare(b.due_time || '');
    });
    
    groups.completed.sort((a, b) => {
        return (b.updated_at || '').localeCompare(a.updated_at || '');
    });
    
    container.innerHTML = `
        <div class="task-group">
            <div class="task-group-title">⏳ Активные задачи (${groups.pending.length})</div>
            ${groups.pending.length === 0 ? '<div class="empty-events">✨ Все задачи выполнены!</div>' : ''}
            ${groups.pending.map(task => renderTaskItem(task)).join('')}
        </div>
        ${groups.completed.length > 0 ? `
            <div class="task-group">
                <div class="task-group-title">✅ Выполненные (${groups.completed.length})</div>
                ${groups.completed.map(task => renderTaskItem(task)).join('')}
            </div>
        ` : ''}
    `;
}

function renderTaskItem(task) {
    return `
        <div class="task-item" data-task-id="${task.id}">
            <div class="task-checkbox ${task.completed ? 'completed' : ''}" onclick="toggleTaskComplete('${task.id}')"></div>
            <div class="task-content">
                <div class="task-title ${task.completed ? 'completed' : ''}">${escapeHtml(task.title)}</div>
                ${task.description ? `<div style="font-size:11px; color:var(--text-muted); margin-top:4px;">${escapeHtml(task.description)}</div>` : ''}
                <div class="task-meta">
                    ${task.due_date ? `<span class="task-date">📅 ${formatDisplayDateStr(task.due_date)}</span>` : ''}
                    ${task.due_time ? `<span class="task-time">⏰ ${task.due_time}</span>` : ''}
                    <span class="task-priority ${task.priority}">${getPriorityIcon(task.priority)} ${getPriorityName(task.priority)}</span>
                </div>
            </div>
            ${window.isCurrentUserAdmin ? `
                <div class="task-actions">
                    <button class="task-action-btn" onclick="editTask('${task.id}')">✏️</button>
                    <button class="task-action-btn" onclick="deleteTask('${task.id}')">🗑️</button>
                </div>
            ` : ''}
        </div>
    `;
}

function formatDisplayDateStr(dateStr) {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    const months = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек'];
    return `${date.getDate()} ${months[date.getMonth()]}`;
}

// Вспомогательные функции для приоритетов
function getPriorityIcon(priority) {
    const icons = { high: '🔴', medium: '🟡', low: '🟢' };
    return icons[priority] || '⚪';
}

function getPriorityName(priority) {
    const names = { high: 'Высокий', medium: 'Средний', low: 'Низкий' };
    return names[priority] || 'Обычный';
}

// Функции для работы с датами
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

function getDayName(date) {
    const days = ['ПН', 'ВТ', 'СР', 'ЧТ', 'ПТ', 'СБ', 'ВС'];
    return days[date.getDay() === 0 ? 6 : date.getDay() - 1];
}

// CRUD операции
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
    
    if (!title) {
        alert('Введите название задачи');
        return;
    }
    
    const data = { title, description, due_date: dueDate, due_time: dueTime, priority };
    
    try {
        let res;
        if (editingTaskId) {
            res = await fetch(`/api/schedule/tasks/${editingTaskId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
                body: JSON.stringify(data)
            });
        } else {
            res = await fetch('/api/schedule/tasks', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
                body: JSON.stringify(data)
            });
        }
        
        if (res.ok) {
            closeTaskModal();
            loadTasks();
            showToast(editingTaskId ? 'Задача обновлена' : 'Задача добавлена');
        } else {
            const error = await res.json();
            alert('Ошибка: ' + (error.error || 'Неизвестная ошибка'));
        }
    } catch(e) {
        console.error('Error saving task:', e);
        alert('Ошибка сохранения');
    }
}

async function toggleTaskComplete(taskId) {
    const token = getToken();
    const task = tasksList.find(t => t.id === taskId);
    if (!task) return;
    
    try {
        const res = await fetch(`/api/schedule/tasks/${taskId}/toggle`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ completed: !task.completed })
        });
        
        if (res.ok) {
            loadTasks();
            showToast(task.completed ? 'Задача возвращена' : 'Задача выполнена! 🎉');
        }
    } catch(e) {
        console.error('Error toggling task:', e);
    }
}

async function markTaskComplete(taskId) {
    if (!confirm('Отметить задачу как выполненную?')) return;
    await toggleTaskComplete(taskId);
}

async function deleteTask(taskId) {
    if (!confirm('Удалить задачу?')) return;
    
    const token = getToken();
    try {
        const res = await fetch(`/api/schedule/tasks/${taskId}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        
        if (res.ok) {
            loadTasks();
            showToast('Задача удалена');
        }
    } catch(e) {
        console.error('Error deleting task:', e);
    }
}

function closeTaskModal() {
    document.getElementById('taskModal').style.display = 'none';
    editingTaskId = null;
}

// Навигация
function changeWeek(delta) {
    currentWeekOffset += delta;
    renderCalendarView();
}

function goToToday() {
    currentWeekOffset = 0;
    renderCalendarView();
}

// Инициализация при переключении вкладки
function initScheduleTab() {
    loadTasks();
    if (window.isCurrentUserAdmin) {
        const adminControls = document.getElementById('adminScheduleControls');
        if (adminControls) adminControls.style.display = 'block';
    }
}
function quickAddTask() {
    const title = document.getElementById('quickTaskTitle').value.trim();
    if (!title) {
        alert('Введите название задачи');
        return;
    }
    
    const priority = document.getElementById('quickTaskPriority').value;
    const dueDate = formatDate(new Date());
    
    saveTaskQuick(title, priority, dueDate);
    document.getElementById('quickTaskTitle').value = '';
}

async function saveTaskQuick(title, priority, dueDate) {
    const token = getToken();
    const data = { title, priority, due_date: dueDate, description: '', due_time: '' };
    
    try {
        const res = await fetch('/api/schedule/tasks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify(data)
        });
        
        if (res.ok) {
            loadTasks();
            showToast('Задача добавлена');
        }
    } catch(e) {
        console.error('Error:', e);
    }
}