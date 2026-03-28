// Общие утилиты
function getToken() { return localStorage.getItem('token'); }

function formatFileSize(b) { 
    if (!b) return '0 B'; 
    const s = ['B', 'KB', 'MB', 'GB']; 
    let i = 0; 
    while (b >= 1024 && i < s.length-1) { 
        b /= 1024; 
        i++; 
    } 
    return b.toFixed(1) + ' ' + s[i]; 
}

function getFileIcon(mime) { 
    if (!mime) return '📄'; 
    if (mime.startsWith('image/')) return '🖼️'; 
    if (mime.startsWith('video/')) return '🎬'; 
    if (mime.startsWith('audio/')) return '🎵'; 
    return '📄'; 
}

function escapeHtml(str) { 
    if (!str) return ''; 
    return str.replace(/[&<>]/g, m => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[m])); 
}

function showToast(message, duration = 2000) {
    let toast = document.getElementById('moveToast');
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'moveToast';
        toast.style.cssText = `
            position: fixed;
            bottom: 80px;
            left: 50%;
            transform: translateX(-50%);
            background: var(--accent-primary);
            color: white;
            padding: 10px 20px;
            border-radius: 30px;
            font-size: 13px;
            z-index: 2000;
            white-space: nowrap;
        `;
        document.body.appendChild(toast);
    }
    toast.textContent = message;
    toast.style.display = 'block';
    setTimeout(() => {
        toast.style.opacity = '0';
        setTimeout(() => toast.style.display = 'none', 300);
    }, duration);
}

function toggleSidebar() { 
    document.getElementById('sidebar').classList.toggle('active'); 
}

function logout() { 
    localStorage.clear(); 
    window.location.href = '/'; 
}

const gradients = [
    'linear-gradient(135deg, #6e4aff, #9b6eff)',
    'linear-gradient(135deg, #ff6b4a, #ff9b6e)',
    'linear-gradient(135deg, #4aff9b, #6effce)',
    'linear-gradient(135deg, #ff4a9b, #ff6ece)',
    'linear-gradient(135deg, #4a9bff, #6eceff)',
    'linear-gradient(135deg, #ffd64a, #ffec6e)'
];
function getGradient(i) { return gradients[i % gradients.length]; }
window.isTouchDevice = ('ontouchstart' in window) || (navigator.maxTouchPoints > 0);