// ==================== РАСПИСАНИЕ МЕРОПРИЯТИЙ ====================
let currentWeekOffset = 0;
let eventsList = [];
let editingEventId = null;

// Получить даты текущей недели
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

// Форматировать дату
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

// Загрузить события
async function loadEvents() {
    const token = getToken();
    if (!token) return;
    
    try {
        const res = await fetch('/api/schedule/events', {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (res.ok) {
            eventsList = await res.json();
            renderSchedule();
        }
    } catch(e) {
        console.error('Error loading events:', e);
    }
}

// Рендер расписания
function renderSchedule() {
    const weekDates = getWeekDates(currentWeekOffset);
    const container = document.getElementById('scheduleGrid');
    if (!container) return;
    
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    
    // Обновляем заголовок недели
    const startWeek = weekDates[0];
    const endWeek = weekDates[6];
    const weekLabel = `${formatDisplayDate(startWeek)} - ${formatDisplayDate(endWeek)}`;
    const weekLabelEl = document.getElementById('currentWeekLabel');
    if (weekLabelEl) weekLabelEl.textContent = weekLabel;
    
    container.innerHTML = weekDates.map(date => {
        const dateStr = formatDate(date);
        const isToday = dateStr === formatDate(today);
        const dayEvents = eventsList.filter(e => e.event_date === dateStr);
        
        return `
            <div class="schedule-day ${isToday ? 'day-today' : ''}">
                <div class="day-header">
                    <div class="day-name">${getDayName(date)}</div>
                    <div class="day-date">${formatDisplayDate(date)}</div>
                </div>
                <div class="day-events">
                    ${dayEvents.length === 0 ? 
                        '<div class="empty-events">📭 Нет мероприятий</div>' :
                        dayEvents.map(event => `
                            <div class="event-card" data-event-id="${event.id}">
                                <div class="event-time">
                                    <span>🕐</span> ${event.event_time || 'Весь день'}
                                </div>
                                <div class="event-title">${escapeHtml(event.title)}</div>
                                ${event.description ? `<div class="event-description">${escapeHtml(event.description)}</div>` : ''}
                                <div class="event-type ${event.event_type}">
                                    ${getEventTypeIcon(event.event_type)} ${getEventTypeName(event.event_type)}
                                </div>
                                ${window.isCurrentUserAdmin ? `
                                    <div class="event-actions">
                                        <button class="event-edit" onclick="editEvent('${event.id}')">✏️ Изменить</button>
                                        <button class="event-delete" onclick="deleteEvent('${event.id}')">🗑️ Удалить</button>
                                    </div>
                                ` : ''}
                            </div>
                        `).join('')
                    }
                </div>
            </div>
        `;
    }).join('');
}

function getEventTypeIcon(type) {
    const icons = {
        'meeting': '🎯',
        'deadline': '⏰',
        'reminder': '🔔',
        'other': '📌'
    };
    return icons[type] || '📌';
}

function getEventTypeName(type) {
    const names = {
        'meeting': 'Встреча',
        'deadline': 'Дедлайн',
        'reminder': 'Напоминание',
        'other': 'Другое'
    };
    return names[type] || 'Другое';
}

// Навигация по неделям
function changeWeek(delta) {
    currentWeekOffset += delta;
    loadEvents();
}

function goToToday() {
    currentWeekOffset = 0;
    loadEvents();
}

// Показать модальное окно добавления события
function showAddEventModal() {
    editingEventId = null;
    document.getElementById('eventModalTitle').textContent = '➕ Добавить событие';
    document.getElementById('eventTitle').value = '';
    document.getElementById('eventDate').value = formatDate(new Date());
    document.getElementById('eventTime').value = '';
    document.getElementById('eventDescription').value = '';
    document.getElementById('eventType').value = 'meeting';
    document.getElementById('eventModal').style.display = 'flex';
}

// Редактировать событие
async function editEvent(eventId) {
    const event = eventsList.find(e => e.id === eventId);
    if (!event) return;
    
    editingEventId = eventId;
    document.getElementById('eventModalTitle').textContent = '✏️ Редактировать событие';
    document.getElementById('eventTitle').value = event.title;
    document.getElementById('eventDate').value = event.event_date;
    document.getElementById('eventTime').value = event.event_time || '';
    document.getElementById('eventDescription').value = event.description || '';
    document.getElementById('eventType').value = event.event_type || 'other';
    document.getElementById('eventModal').style.display = 'flex';
}

// Сохранить событие
async function saveEvent() {
    const token = getToken();
    const title = document.getElementById('eventTitle').value.trim();
    const eventDate = document.getElementById('eventDate').value;
    const eventTime = document.getElementById('eventTime').value;
    const description = document.getElementById('eventDescription').value.trim();
    const eventType = document.getElementById('eventType').value;
    
    if (!title) {
        alert('Введите название мероприятия');
        return;
    }
    if (!eventDate) {
        alert('Выберите дату');
        return;
    }
    
    const data = { title, event_date: eventDate, event_time: eventTime, description, event_type: eventType };
    
    try {
        let res;
        if (editingEventId) {
            res = await fetch(`/api/schedule/events/${editingEventId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
                body: JSON.stringify(data)
            });
        } else {
            res = await fetch('/api/schedule/events', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
                body: JSON.stringify(data)
            });
        }
        
        if (res.ok) {
            closeEventModal();
            loadEvents();
            showToast(editingEventId ? 'Событие обновлено' : 'Событие добавлено');
        } else {
            const error = await res.json();
            alert('Ошибка: ' + (error.error || 'Неизвестная ошибка'));
        }
    } catch(e) {
        console.error('Error saving event:', e);
        alert('Ошибка сохранения');
    }
}

// Удалить событие
async function deleteEvent(eventId) {
    if (!confirm('Удалить это мероприятие?')) return;
    
    const token = getToken();
    try {
        const res = await fetch(`/api/schedule/events/${eventId}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        
        if (res.ok) {
            loadEvents();
            showToast('Событие удалено');
        } else {
            const error = await res.json();
            alert('Ошибка: ' + (error.error || 'Неизвестная ошибка'));
        }
    } catch(e) {
        console.error('Error deleting event:', e);
        alert('Ошибка удаления');
    }
}

function closeEventModal() {
    document.getElementById('eventModal').style.display = 'none';
    editingEventId = null;
}

// Переключение между вкладками
function switchContentTab(tab) {
    // Обновляем кнопки
    document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
    document.querySelector(`.tab-btn[data-tab="${tab}"]`).classList.add('active');
    
    // Обновляем контент
    const filesContent = document.getElementById('filesContent');
    const scheduleContent = document.getElementById('scheduleContent');
    
    if (filesContent) filesContent.style.display = tab === 'files' ? 'block' : 'none';
    if (scheduleContent) scheduleContent.style.display = tab === 'schedule' ? 'block' : 'none';
    
    // Если переключились на расписание - загружаем события
    if (tab === 'schedule') {
        loadEvents();
        // Показываем кнопки админа если нужно
        const adminControls = document.getElementById('adminScheduleControls');
        if (adminControls) {
            adminControls.style.display = window.isCurrentUserAdmin ? 'block' : 'none';
        }
    }
}