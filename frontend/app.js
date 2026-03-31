const tg = window.Telegram.WebApp;
const API_BASE = window.location.origin;

let currentSessionId = null;
let telegramUserId = null;
let isLoading = false;
let allSessions = [];
let currentTab = 'chat';
let lastGeneratedImageUrl = null;

// ========== INIT ==========
document.addEventListener("DOMContentLoaded", () => {
    tg.ready();
    tg.expand();

    const user = tg.initDataUnsafe?.user;
    telegramUserId = user?.id || 12345;

    if (user?.first_name) {
        const el = document.getElementById("tg-user-name");
        if (el) el.textContent = user.first_name;
    }

    currentSessionId = generateSessionId();
    loadSessions();
    setupChatListeners();
});

// ========== TABS ==========
window.switchTab = function(tab) {
    currentTab = tab;
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
    document.getElementById('tab-' + tab).classList.add('active');
    document.getElementById('content-' + tab).classList.add('active');
};

// ========== CHAT ==========
function setupChatListeners() {
    const messageInput = document.getElementById("message-input");
    const sendBtn = document.getElementById("send-btn");

    document.getElementById("history-btn").addEventListener("click", openSidebar);
    document.getElementById("close-sidebar-btn").addEventListener("click", closeSidebar);
    document.getElementById("history-overlay").addEventListener("click", closeSidebar);
    document.getElementById("new-chat-btn").addEventListener("click", startNewChat);

    sendBtn.addEventListener("click", sendMessage);
    messageInput.addEventListener("keypress", (e) => {
        if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendMessage(); }
    });
    messageInput.addEventListener("input", function() {
        this.style.height = "auto";
        this.style.height = Math.min(this.scrollHeight, 120) + "px";
        if (!this.value) this.style.height = "auto";
    });
}

async function sendMessage() {
    if (isLoading) return;
    const messageInput = document.getElementById("message-input");
    const text = messageInput.value.trim();
    if (!text) return;

    const welcomeMsg = document.getElementById("welcome-msg");
    if (welcomeMsg) welcomeMsg.remove();

    appendBubble(text, "user");
    messageInput.value = "";
    messageInput.style.height = "auto";
    scrollToBottom();

    isLoading = true;
    document.getElementById("send-btn").disabled = true;
    showTypingIndicator();

    try {
        const res = await fetch(`${API_BASE}/api/chat`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ telegram_id: telegramUserId, session_id: currentSessionId, message: text }),
        });
        const data = await res.json();
        hideTypingIndicator();

        if (data.reply) {
            appendBubble(data.reply, "ai");
            if (tg.HapticFeedback) tg.HapticFeedback.impactOccurred("light");
            await loadSessions();
            if (document.getElementById("current-chat-title").textContent === "Yangi suhbat") {
                const t = text.length > 35 ? text.substring(0, 35) + "..." : text;
                document.getElementById("current-chat-title").textContent = t;
            }
        } else {
            appendBubble("❌ Xato: " + (data.error || "noma'lum"), "ai");
        }
    } catch (err) {
        hideTypingIndicator();
        appendBubble("❌ Server bilan ulanishda xato.", "ai");
    } finally {
        isLoading = false;
        document.getElementById("send-btn").disabled = false;
        scrollToBottom();
    }
}

// ========== IMAGE GENERATION ==========
window.addStyle = function(style) {
    const ta = document.getElementById("image-prompt");
    const cur = ta.value.trim();
    ta.value = cur ? cur + ", " + style : style;
    ta.focus();
};

window.generateImage = async function() {
    const prompt = document.getElementById("image-prompt").value.trim();
    if (!prompt) {
        showImageError("Iltimos, prompt kiriting!");
        return;
    }

    const sendToTg = document.getElementById("send-to-tg").checked;

    document.getElementById("image-result").style.display = "none";
    document.getElementById("image-error").style.display = "none";
    document.getElementById("image-loading").style.display = "flex";
    document.getElementById("generate-btn").disabled = true;

    try {
        const res = await fetch(`${API_BASE}/api/generate-image`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                prompt: prompt,
                telegram_id: telegramUserId,
                send_to_bot: sendToTg,
            }),
        });
        const data = await res.json();

        if (data.image_url) {
            lastGeneratedImageUrl = data.image_url;
            document.getElementById("result-img").src = data.image_url + "?t=" + Date.now();
            document.getElementById("image-result").style.display = "block";
            if (tg.HapticFeedback) tg.HapticFeedback.notificationOccurred("success");
        } else {
            showImageError(data.error || "Rasm yaratib bo'lmadi");
        }
    } catch (err) {
        showImageError("Server bilan ulanishda xato: " + err.message);
    } finally {
        document.getElementById("image-loading").style.display = "none";
        document.getElementById("generate-btn").disabled = false;
    }
};

window.downloadImage = function() {
    if (!lastGeneratedImageUrl) return;
    const a = document.createElement("a");
    a.href = lastGeneratedImageUrl;
    a.download = "ai-generated.png";
    a.click();
};

window.sendResultToTelegram = async function() {
    if (!lastGeneratedImageUrl) return;
    const prompt = document.getElementById("image-prompt").value.trim();
    const btn = event.target.closest('button');
    btn.disabled = true;
    btn.textContent = "Yuborilmoqda...";

    try {
        await fetch(`${API_BASE}/api/generate-image`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                prompt: prompt,
                telegram_id: telegramUserId,
                send_to_bot: true,
            }),
        });
        btn.textContent = "✅ Yuborildi!";
        setTimeout(() => { btn.disabled = false; btn.innerHTML = '📤 TG ga yuborish'; }, 2000);
    } catch {
        btn.textContent = "❌ Xato";
        btn.disabled = false;
    }
};

function showImageError(msg) {
    const el = document.getElementById("image-error");
    el.textContent = "❌ " + msg;
    el.style.display = "block";
}

// ========== VISION ==========
window.previewImage = function(event) {
    const file = event.target.files[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (e) => {
        const img = document.getElementById("preview-img");
        img.src = e.target.result;
        img.style.display = "block";
        document.getElementById("upload-placeholder").style.display = "none";
        document.getElementById("upload-area").classList.add("has-image");
        document.getElementById("analyze-btn").style.display = "flex";
        document.getElementById("vision-result").style.display = "none";
    };
    reader.readAsDataURL(file);
};

window.analyzeImage = async function() {
    const img = document.getElementById("preview-img");
    if (!img.src || img.src === window.location.href) return;

    const question = document.getElementById("vision-question").value.trim();

    document.getElementById("vision-loading").style.display = "flex";
    document.getElementById("vision-result").style.display = "none";
    document.getElementById("analyze-btn").disabled = true;

    try {
        const res = await fetch(`${API_BASE}/api/analyze-image`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                image_base64: img.src,
                question: question,
                telegram_id: telegramUserId,
            }),
        });
        const data = await res.json();

        if (data.analysis) {
            document.getElementById("vision-text").textContent = data.analysis;
            document.getElementById("vision-result").style.display = "block";
            if (tg.HapticFeedback) tg.HapticFeedback.impactOccurred("light");
        } else {
            alert("❌ " + (data.error || "Tahlil qilib bo'lmadi"));
        }
    } catch (err) {
        alert("❌ Server xato: " + err.message);
    } finally {
        document.getElementById("vision-loading").style.display = "none";
        document.getElementById("analyze-btn").disabled = false;
    }
};

// ========== SESSIONS ==========
async function loadSessions() {
    try {
        const res = await fetch(`${API_BASE}/api/sessions?telegram_id=${telegramUserId}`);
        const data = await res.json();
        allSessions = data.sessions || [];
        renderSessionsList();
        const badge = document.getElementById("history-badge");
        if (allSessions.length > 0) {
            badge.textContent = allSessions.length > 9 ? "9+" : allSessions.length;
            badge.classList.add("show");
        } else {
            badge.classList.remove("show");
        }
    } catch {}
}

function renderSessionsList() {
    const list = document.getElementById("sessions-list");
    if (allSessions.length === 0) {
        list.innerHTML = `<div class="no-sessions"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="width:40px;height:40px;display:block;margin:0 auto 12px;opacity:0.4"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path></svg>Hozircha suhbat yo'q.</div>`;
        return;
    }
    const groups = groupByDate(allSessions);
    let html = "";
    for (const [label, sessions] of Object.entries(groups)) {
        html += `<div class="session-date-label">${label}</div>`;
        for (const s of sessions) {
            const isActive = s.SessionID === currentSessionId;
            html += `<div class="session-item ${isActive ? "active" : ""}" onclick="loadSession('${s.SessionID}', '${escapeHtml(s.Title)}')">
                <div class="session-icon">💬</div>
                <div class="session-info">
                    <div class="session-title">${escapeHtml(s.Title || "Suhbat")}</div>
                    <div class="session-time">${formatTime(s.UpdatedAt)}</div>
                </div>
                <button class="session-delete" onclick="deleteSession(event, '${s.SessionID}')">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14H6L5 6"/></svg>
                </button>
            </div>`;
        }
    }
    list.innerHTML = html;
}

window.loadSession = async function(sessionId, title) {
    currentSessionId = sessionId;
    document.getElementById("current-chat-title").textContent = title || "Suhbat";
    const chatContainer = document.getElementById("chat-container");
    chatContainer.innerHTML = "";
    try {
        const res = await fetch(`${API_BASE}/api/history?session_id=${sessionId}`);
        const data = await res.json();
        const messages = data.messages || [];
        if (messages.length === 0) { showWelcome(); }
        else { messages.forEach(m => appendBubble(m.Content, m.Role === "user" ? "user" : "ai", false)); }
    } catch { showWelcome(); }
    scrollToBottom();
    closeSidebar();
    renderSessionsList();
    switchTab('chat');
};

window.deleteSession = async function(event, sessionId) {
    event.stopPropagation();
    if (!confirm("Bu suhbatni o'chirasizmi?")) return;
    await fetch(`${API_BASE}/api/session/${sessionId}`, { method: "DELETE" });
    if (sessionId === currentSessionId) startNewChat();
    await loadSessions();
};

function startNewChat() {
    currentSessionId = generateSessionId();
    document.getElementById("current-chat-title").textContent = "Yangi suhbat";
    document.getElementById("chat-container").innerHTML = "";
    showWelcome();
    closeSidebar();
    renderSessionsList();
}

// ========== SIDEBAR ==========
function openSidebar() {
    document.getElementById("history-sidebar").classList.add("open");
    document.getElementById("history-overlay").classList.add("open");
    loadSessions();
}
function closeSidebar() {
    document.getElementById("history-sidebar").classList.remove("open");
    document.getElementById("history-overlay").classList.remove("open");
}

// ========== UI HELPERS ==========
function appendBubble(text, type, animate = true) {
    const chatContainer = document.getElementById("chat-container");
    const bubble = document.createElement("div");
    bubble.className = `message-bubble message-${type}`;
    if (!animate) bubble.style.animation = "none";
    bubble.textContent = text;
    chatContainer.appendChild(bubble);
}

function showWelcome() {
    const chatContainer = document.getElementById("chat-container");
    const user = tg.initDataUnsafe?.user;
    const name = user?.first_name || "Do'stim";
    const div = document.createElement("div");
    div.className = "welcome-message"; div.id = "welcome-msg";
    div.innerHTML = `<span class="emoji">✨</span><h3>Salom, ${escapeHtml(name)}!</h3><p>Groq AI yordamida ishlaydi. Savolingizni yozing!</p>`;
    chatContainer.appendChild(div);
}

function scrollToBottom() {
    const c = document.getElementById("chat-container");
    if (c) setTimeout(() => { c.scrollTop = c.scrollHeight; }, 50);
}

function showTypingIndicator() {
    hideTypingIndicator();
    const c = document.getElementById("chat-container");
    const el = document.createElement("div");
    el.id = "typing"; el.className = "typing-indicator";
    el.innerHTML = "<div class='dot'></div><div class='dot'></div><div class='dot'></div>";
    c.appendChild(el); scrollToBottom();
}
function hideTypingIndicator() {
    const el = document.getElementById("typing");
    if (el) el.remove();
}

// ========== UTILS ==========
function generateSessionId() { return "sess_" + Date.now() + "_" + Math.random().toString(36).substr(2, 9); }
function escapeHtml(text) {
    if (!text) return "";
    const d = document.createElement("div");
    d.appendChild(document.createTextNode(text));
    return d.innerHTML;
}
function formatTime(dateStr) {
    if (!dateStr) return "";
    const date = new Date(dateStr);
    const diff = Math.floor((Date.now() - date) / 86400000);
    if (diff === 0) return date.toLocaleTimeString("uz-UZ", { hour: "2-digit", minute: "2-digit" });
    if (diff === 1) return "Kecha";
    if (diff < 7) return diff + " kun oldin";
    return date.toLocaleDateString("uz-UZ", { day: "numeric", month: "short" });
}
function groupByDate(sessions) {
    const groups = {};
    const now = Date.now();
    for (const s of sessions) {
        const diff = Math.floor((now - new Date(s.UpdatedAt || s.CreatedAt)) / 86400000);
        const label = diff === 0 ? "Bugun" : diff === 1 ? "Kecha" : diff < 7 ? "Bu hafta" : diff < 30 ? "Bu oy" : "Oldinroq";
        if (!groups[label]) groups[label] = [];
        groups[label].push(s);
    }
    return groups;
}