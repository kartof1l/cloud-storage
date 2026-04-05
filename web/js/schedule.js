// ==================== ТАСК-МЕНЕДЖЕР ====================
let currentWeekOffset = 0;
let tasksList = [];
let editingTaskId = null;
let currentView = 'calendar';

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

// Переключение вида
function switchView(view) {
    currentView = view;
    
    document.querySelectorAll('.view-btn').forEach(btn => btn.classList.remove('active'));
    const activeBtn = document.querySelector(`.view-btn[data-view="${view}"]`);
    if (activeBtn) activeBtn.classList.add('active');
    
    const calendarView = document.getElementById('calendarView');
    const listView = document.getElementById('listView');
    
    if (calendarView) calendarView.classList.toggle('active', view === 'calendar');
    if (listView) listView.classList.toggle('active', view === 'list');
    
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

// Проверка просрочена ли задача
function isTaskOverdue(task) {
    if (task.completed) return false;
    if (!task.due_date) return false;
    const today = formatDate(new Date());
    return task.due_date < today;
}

// Рендер календарного вида
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
    
    // Находим сегодняшний индекс для анимации
    const todayIndex = weekDates.findIndex(d => formatDate(d) === todayStr);
    
    container.innerHTML = weekDates.map((date, idx) => {
        const dateStr = formatDate(date);
        const isToday = dateStr === todayStr;
        const dayTasks = tasksList.filter(t => t.due_date === dateStr);
        
        // Сортируем задачи
        dayTasks.sort((a, b) => {
            if (a.completed !== b.completed) return a.completed ? 1 : -1;
            const priorityOrder = { high: 0, medium: 1, low: 2 };
            return (priorityOrder[a.priority] || 3) - (priorityOrder[b.priority] || 3);
        });
        
        // Анимация для сегодняшнего дня
        const todayAnimation = isToday ? 'style="animation: todayGlow 3s ease-in-out infinite; position: relative; overflow: hidden;"' : '';
        const todayClockAnimation = isToday ? '<div class="today-clock"></div>' : '';
        
        return `
            <div class="schedule-day ${isToday ? 'day-today' : ''}" ${todayAnimation}>
                ${todayClockAnimation}
                <div class="day-header">
                    <div class="day-name">${getDayName(date)}</div>
                    <div class="day-date">${formatDisplayDate(date)}</div>
                </div>
                <div class="day-events">
                    ${window.isCurrentUserAdmin ? `
                        <button class="btn btn-primary add-task-btn" style="width:100%; margin-bottom:12px; padding:8px;" onclick="showTaskModal('${dateStr}')">
                            + Добавить задачу
                        </button>
                    ` : ''}
                    ${dayTasks.length === 0 ? 
                        '<div class="empty-events">📭 Нет задач</div>' :
                        dayTasks.map(task => {
                            const isOverdue = isTaskOverdue(task);
                            return `
                                <div class="task-card ${task.completed ? 'completed-task' : ''} ${isOverdue ? 'overdue-task' : ''}" onclick="event.stopPropagation()">
                                    <div class="task-card-time">
                                        <span>🕐</span> ${task.due_time || 'Любое время'}
                                        ${isOverdue ? '<span style="color:var(--danger); margin-left:8px;">⚠️ Просрочено</span>' : ''}
                                        ${task.completed ? '<span style="color:var(--success); margin-left:8px;">✅ Выполнено</span>' : ''}
                                    </div>
                                    <div class="task-card-title ${task.completed ? 'completed' : ''}">
                                        ${escapeHtml(task.title)}
                                    </div>
                                    ${task.description ? `<div style="font-size:10px; color:var(--text-muted); margin-bottom:6px;">${escapeHtml(task.description)}</div>` : ''}
                                    <div style="display:flex; justify-content:space-between; align-items:center;">
                                        <span class="task-card-priority priority-${task.priority}">
                                            ${getPriorityIcon(task.priority)} ${getPriorityName(task.priority)}
                                        </span>
                                        <div style="display:flex; gap:6px;">
                                            ${!task.completed ? 
                                                `<button class="task-action-btn complete-btn" onclick="toggleTaskComplete('${task.id}')">
                                                    ✅ Выполнить
                                                </button>` :
                                                `<button class="task-action-btn revert-btn" onclick="toggleTaskComplete('${task.id}')">
                                                    ↩️ Вернуть
                                                </button>`
                                            }
                                            ${window.isCurrentUserAdmin ? `
                                                <button class="task-action-btn edit-btn" onclick="editTask('${task.id}')">✏️</button>
                                                <button class="task-action-btn delete-btn" onclick="deleteTask('${task.id}')">🗑️</button>
                                            ` : ''}
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
    
    // Добавляем стили для анимации если их нет
    if (!document.querySelector('#todayAnimationStyle')) {
        const style = document.createElement('style');
        style.id = 'todayAnimationStyle';
        style.textContent = `
            @keyframes todayGlow {
                0% { box-shadow: 0 0 0 0 rgba(110, 74, 255, 0.4); border-color: var(--accent-primary); }
                50% { box-shadow: 0 0 0 15px rgba(110, 74, 255, 0); border-color: var(--accent-secondary); }
                100% { box-shadow: 0 0 0 0 rgba(110, 74, 255, 0); border-color: var(--accent-primary); }
            }
            .today-clock {
                position: absolute;
                top: 10px;
                right: 10px;
                width: 30px;
                height: 30px;
                border-radius: 50%;
                background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
                animation: clockRotate 2s linear infinite;
                opacity: 0.6;
            }
            @keyframes clockRotate {
                0% { transform: rotate(0deg); }
                100% { transform: rotate(360deg); }
            }
            .today-clock::before {
                content: '';
                position: absolute;
                top: 50%;
                left: 50%;
                width: 2px;
                height: 12px;
                background: white;
                transform: translate(-50%, -50%);
                border-radius: 2px;
            }
            .today-clock::after {
                content: '';
                position: absolute;
                top: 50%;
                left: 50%;
                width: 8px;
                height: 2px;
                background: white;
                transform: translate(-50%, -50%);
                border-radius: 2px;
            }
            .completed-task {
                opacity: 0.7;
                background: rgba(0, 214, 143, 0.1);
                border-left-color: var(--success);
            }
            .overdue-task {
                border-left-color: var(--danger);
                background: rgba(255, 77, 109, 0.05);
            }
            .complete-btn { background: var(--success); color: white; }
            .revert-btn { background: var(--warning); color: white; }
            .edit-btn { background: var(--accent-primary); color: white; }
            .delete-btn { background: var(--danger); color: white; }
            .add-task-btn {
                background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
                color: white;
                border: none;
            }
        `;
        document.head.appendChild(style);
    }
}

// Рендер спискового вида
function renderListView() {
    const container = document.getElementById('tasksListContainer');
    if (!container) return;
    
    const groups = {
        overdue: tasksList.filter(t => !t.completed && isTaskOverdue(t)),
        pending: tasksList.filter(t => !t.completed && !isTaskOverdue(t)),
        completed: tasksList.filter(t => t.completed)
    };
    
    // Сортируем задачи
    groups.overdue.sort((a, b) => (a.due_date || '').localeCompare(b.due_date || ''));
    groups.pending.sort((a, b) => {
        const priorityOrder = { high: 0, medium: 1, low: 2 };
        if (a.priority !== b.priority) return (priorityOrder[a.priority] || 3) - (priorityOrder[b.priority] || 3);
        if (a.due_date !== b.due_date) return (a.due_date || '9999-99-99').localeCompare(b.due_date || '9999-99-99');
        return (a.due_time || '').localeCompare(b.due_time || '');
    });
    groups.completed.sort((a, b) => (b.updated_at || '').localeCompare(a.updated_at || ''));
    
    container.innerHTML = `
        ${groups.overdue.length > 0 ? `
            <div class="task-group">
                <div class="task-group-title" style="background: rgba(255,77,109,0.2); color: var(--danger);">⚠️ Просроченные (${groups.overdue.length})</div>
                ${groups.overdue.map(task => renderTaskItem(task, true)).join('')}
            </div>
        ` : ''}
        <div class="task-group">
            <div class="task-group-title">⏳ Активные задачи (${groups.pending.length})</div>
            ${groups.pending.length === 0 && groups.overdue.length === 0 ? '<div class="empty-events">✨ Все задачи выполнены!</div>' : ''}
            ${groups.pending.map(task => renderTaskItem(task, false)).join('')}
        </div>
        ${groups.completed.length > 0 ? `
            <div class="task-group">
                <div class="task-group-title">✅ Выполненные (${groups.completed.length})</div>
                ${groups.completed.map(task => renderTaskItem(task, false)).join('')}
            </div>
        ` : ''}
    `;
}

function renderTaskItem(task, isOverdue = false) {
    return `
        <div class="task-item ${task.completed ? 'completed' : ''} ${isOverdue ? 'overdue' : ''}" data-task-id="${task.id}">
            <div class="task-checkbox ${task.completed ? 'completed' : ''}" onclick="toggleTaskComplete('${task.id}')"></div>
            <div class="task-content">
                <div class="task-title ${task.completed ? 'completed' : ''}">${escapeHtml(task.title)}</div>
                ${task.description ? `<div style="font-size:11px; color:var(--text-muted); margin-top:4px;">${escapeHtml(task.description)}</div>` : ''}
                <div class="task-meta">
                    ${task.due_date ? `<span class="task-date">📅 ${formatDisplayDateStr(task.due_date)}${isOverdue ? ' (просрочено)' : ''}</span>` : ''}
                    ${task.due_time ? `<span class="task-time">⏰ ${task.due_time}</span>` : ''}
                    <span class="task-priority ${task.priority}">${getPriorityIcon(task.priority)} ${getPriorityName(task.priority)}</span>
                </div>
            </div>
            <div class="task-actions">
                ${!task.completed ? 
                    `<button class="task-action-btn complete-btn" onclick="toggleTaskComplete('${task.id}')">✅ Выполнить</button>` :
                    `<button class="task-action-btn revert-btn" onclick="toggleTaskComplete('${task.id}')">↩️ Вернуть</button>`
                }
                ${window.isCurrentUserAdmin ? `
                    <button class="task-action-btn edit-btn" onclick="editTask('${task.id}')">✏️</button>
                    <button class="task-action-btn delete-btn" onclick="deleteTask('${task.id}')">🗑️</button>
                ` : ''}
            </div>
        </div>
    `;
}

// CRUD операции
function showTaskModal(prefilledDate = '') {
    editingTaskId = null;
    const modalTitle = document.getElementById('taskModalTitle');
    const titleInput = document.getElementById('taskTitle');
    const descInput = document.getElementById('taskDescription');
    const dateInput = document.getElementById('taskDueDate');
    const timeInput = document.getElementById('taskDueTime');
    const prioritySelect = document.getElementById('taskPriority');
    
    if (modalTitle) modalTitle.textContent = '➕ Новая задача';
    if (titleInput) titleInput.value = '';
    if (descInput) descInput.value = '';
    if (dateInput) dateInput.value = prefilledDate || formatDate(new Date());
    if (timeInput) timeInput.value = '';
    if (prioritySelect) prioritySelect.value = 'medium';
    
    const modal = document.getElementById('taskModal');
    if (modal) modal.style.display = 'flex';
}

async function editTask(taskId) {
    const task = tasksList.find(t => t.id === taskId);
    if (!task) return;
    
    editingTaskId = taskId;
    const modalTitle = document.getElementById('taskModalTitle');
    const titleInput = document.getElementById('taskTitle');
    const descInput = document.getElementById('taskDescription');
    const dateInput = document.getElementById('taskDueDate');
    const timeInput = document.getElementById('taskDueTime');
    const prioritySelect = document.getElementById('taskPriority');
    
    if (modalTitle) modalTitle.textContent = '✏️ Редактировать задачу';
    if (titleInput) titleInput.value = task.title;
    if (descInput) descInput.value = task.description || '';
    if (dateInput) dateInput.value = task.due_date || '';
    if (timeInput) timeInput.value = task.due_time || '';
    if (prioritySelect) prioritySelect.value = task.priority || 'medium';
    
    const modal = document.getElementById('taskModal');
    if (modal) modal.style.display = 'flex';
}

async function saveTask() {
    const token = getToken();
    const titleInput = document.getElementById('taskTitle');
    const descInput = document.getElementById('taskDescription');
    const dateInput = document.getElementById('taskDueDate');
    const timeInput = document.getElementById('taskDueTime');
    const prioritySelect = document.getElementById('taskPriority');
    
    const title = titleInput ? titleInput.value.trim() : '';
    const description = descInput ? descInput.value.trim() : '';
    const dueDate = dateInput ? dateInput.value : '';
    const dueTime = timeInput ? timeInput.value : '';
    const priority = prioritySelect ? prioritySelect.value : 'medium';
    
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
            if (typeof showToast === 'function') {
                showToast(editingTaskId ? 'Задача обновлена' : 'Задача добавлена');
            }
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
            if (typeof showToast === 'function') {
                showToast(task.completed ? 'Задача возвращена' : 'Задача выполнена! 🎉');
            }
        }
    } catch(e) {
        console.error('Error toggling task:', e);
    }
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
            if (typeof showToast === 'function') {
                showToast('Задача удалена');
            }
        }
    } catch(e) {
        console.error('Error deleting task:', e);
    }
}

function closeTaskModal() {
    const modal = document.getElementById('taskModal');
    if (modal) modal.style.display = 'none';
    editingTaskId = null;
}

// Навигация по неделям
function changeWeek(delta) {
    currentWeekOffset += delta;
    renderCalendarView();
}

function goToToday() {
    currentWeekOffset = 0;
    renderCalendarView();
    // Прокручиваем к сегодняшнему дню
    setTimeout(() => {
        const todayCard = document.querySelector('.schedule-day.day-today');
        if (todayCard) {
            todayCard.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
    }, 100);
}

// Быстрое добавление
function quickAddTask() {
    const titleInput = document.getElementById('quickTaskTitle');
    const title = titleInput ? titleInput.value.trim() : '';
    if (!title) {
        alert('Введите название задачи');
        return;
    }
    
    const prioritySelect = document.getElementById('quickTaskPriority');
    const priority = prioritySelect ? prioritySelect.value : 'medium';
    const dueDate = formatDate(new Date());
    
    saveTaskQuick(title, priority, dueDate);
    if (titleInput) titleInput.value = '';
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
            if (typeof showToast === 'function') {
                showToast('Задача добавлена');
            }
        }
    } catch(e) {
        console.error('Error:', e);
    }
}

// Переключение вкладок
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
        if (quickAddBlock) {
            quickAddBlock.style.display = window.isCurrentUserAdmin ? 'block' : 'none';
        }
        loadTasks();
    }
}

// Инициализация
document.addEventListener('DOMContentLoaded', function() {
    console.log('Schedule.js loaded');
    
    setTimeout(function() {
        if (window.isCurrentUserAdmin) {
            const quickAddBlock = document.getElementById('quickAddBlock');
            if (quickAddBlock) quickAddBlock.style.display = 'block';
        }
    }, 500);
});