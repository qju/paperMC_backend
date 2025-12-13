function getToken() {
    return localStorage.getItem('token');
}

function getAuthHeaders() {
    return {
        'Authorization': 'Bearer ' + getToken(),
        'Content-Type': 'application/json'
    };
}

function checkAuth() {
    fetch('/status', {
        headers: {
            'Authorization': 'Bearer ' + getToken()
        }
    }).then(response => {
        if (response.status === 401) {
            // Token invalid or missing -> Go to Login
            window.location.href = "/login/";
        }
    }).catch(err => {
        console.error("Auth check failed", err);
    });
}

let socket;

function connectWebSocket() {
    const token = getToken();
    if (!token) {
        console.log("No token found, skipping WS connection");
        return;
    }

    // Determine correct protocol (ws or wss)
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Connect with Token in Query Param (Backend middleware expects this!)
    socket = new WebSocket(`${protocol}//${window.location.host}/ws?token=${token}`);

    socket.onopen = () => {
        addLocalLog("[System] Connected to Real-Time Console.", false);
        // Visual indicator that we are live
        const statusBadge = document.getElementById('status-badge');
        if (statusBadge) statusBadge.classList.remove('stopped');
    };

    socket.onmessage = (event) => {
        // Parse the JSON message from the server
        try {
            const msg = JSON.parse(event.data);

            // Handle Log Lines
            if (msg.type === "log") {
                const newLine = document.createElement("div");
                newLine.innerText = msg.data;

                if (msg.data.includes("ERROR") || msg.data.includes("Exception")) {
                    newLine.style.color = "#ff5555"; // Minecraft Red
                }

                const logContainer = document.getElementById("log-container");
                logContainer.appendChild(newLine);
                logContainer.scrollTop = logContainer.scrollHeight;
            }
        } catch (e) {
            console.error("Error parsing WS message:", e);
        }
    };

    socket.onclose = (event) => {
        addLocalLog("[System] Connection lost. Reconnecting in 3s...", true);
        setTimeout(connectWebSocket, 3000);
    };

    socket.onerror = (error) => {
        console.error("WebSocket Error:", error);
    };
}

// Run immediately when page loads
document.addEventListener("DOMContentLoaded", () => {
    checkAuth();
    connectWebSocket(); // Start the socket!
    updateStatus();     // Get initial status
});

const cmdInput = document.getElementById('cmd-input');
const statusBadge = document.getElementById('status-badge');

function updateStatus() {
    // Added Auth Header
    fetch('/status', {
        headers: { 'Authorization': 'Bearer ' + getToken() }
    })
        .then(response => {
            if (response.status === 401) window.location.href = "/login/";
            return response.json();
        })
        .then(data => {
            statusBadge.innerText = data.status;
            statusBadge.className = "status-badge";
            if (data.status === "Running") {
                statusBadge.classList.add('running');
            } else {
                statusBadge.classList.add('stopped');
            }
        })
        .catch(err => {
            console.error("API Error: ", err);
            statusBadge.classList.add('stopped');
            statusBadge.innerText = "Offline";
        });
}

function startServer() {
    // Added Auth Header
    fetch('/start', {
        method: 'POST',
        headers: getAuthHeaders()
    }).then(updateStatus);
}

function stopServer() {
    // Added Auth Header
    fetch('/stop', {
        method: 'POST',
        headers: getAuthHeaders()
    }).then(updateStatus);
}

function sendCommand() {
    const cmd = cmdInput.value;
    if (!cmd) return;

    // Added Auth Header
    fetch('/command', {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify({ command: cmd })
    }).then(resp => {
        if (resp.ok) {
            cmdInput.value = '';
        } else {
            addLocalLog("System: Command failed to send (401/500).", true);
        }
    });
}

// --- Modals & Config ---

function openModalWhitelist() {
    document.getElementById("modal-overlay").classList.remove('hidden');
    document.getElementById("modal-input").focus();
}

function openModalConfig() {
    document.getElementById("config-modal-overlay").classList.remove('hidden');
    loadConfigData();
}

function openModalUpdate() {
    document.getElementById("update-modal-overlay").classList.remove('hidden');
}

function closeModal() {
    document.getElementById("modal-overlay").classList.add('hidden');
    document.getElementById("modal-input").value = '';
}

function closeModalConfig() {
    document.getElementById("config-modal-overlay").classList.add('hidden');
}

function closeModalUpdate() {
    document.getElementById("update-modal-overlay").classList.add('hidden');
}

function whiteList() {
    const wlInput = document.getElementById('modal-input');
    const cmd = wlInput.value;

    if (!cmd) return;

    // Added Auth Header
    fetch('/whitelist_add', {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify({ command: cmd })
    }).then(resp => {
        if (resp.ok) {
            wlInput.value = '';
            addLocalLog(`[System] Whitelist request sent for: ${cmd}`);
            closeModal();
        } else {
            addLocalLog("System: Command failed to send.", true);
        }
    });
}

function loadConfigData() {
    const container = document.getElementById('config-container');
    container.innerHTML = '<div style="text-align:center;">Loading...</div>';

    // Added Auth Header
    fetch('/config', {
        headers: { 'Authorization': 'Bearer ' + getToken() }
    })
        .then(res => res.json())
        .then(data => {
            container.innerHTML = '';

            Object.keys(data).sort().forEach(key => {
                const row = document.createElement('div');
                row.style.display = 'flex';
                row.style.justifyContent = 'space-between';
                row.style.alignItems = 'center';

                const label = document.createElement('label');
                label.innerText = key;
                label.style.color = '#333';
                label.style.fontFamily = 'monospace';
                label.style.fontSize = "1rem";
                label.style.width = "100%";

                const input = document.createElement('input');
                input.type = 'text';
                input.value = data[key];
                input.className = 'config-input modal-input';
                input.dataset.key = key;

                row.appendChild(label);
                row.appendChild(input);
                container.appendChild(row);
            });
        })
        .catch(err => {
            container.innerHTML = `<div style="color:red">Error loading config: ${err}</div>`;
        });
}

function saveConfig() {
    const inputs = document.querySelectorAll('.config-input');
    const updates = {};

    inputs.forEach(input => {
        updates[input.dataset.key] = input.value;
    });

    // Added Auth Header
    fetch('/config', {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify(updates)
    }).then(resp => {
        if (resp.ok) {
            addLocalLog("[System] Config saved successfully.");
            closeModalConfig();
        } else {
            addLocalLog("[Error] Failed to save config.", true);
        }
    });
}

// Close modal on "Escape" key
document.addEventListener('keydown', function (event) {
    if (event.key === "Escape") {
        closeModal();
        closeModalConfig();
        closeModalUpdate();
    }
});

// --- Helpers ---

function handleEnter(event) {
    if (event.key === 'Enter') {
        sendCommand();
    }
}

function scrollToBottom() {
    const logContainer = document.getElementById("log-container");
    if (logContainer) logContainer.scrollTop = logContainer.scrollHeight;
}

function addLocalLog(msg, isError = false) {
    const logContainer = document.getElementById("log-container");
    if (!logContainer) return;

    const newLine = document.createElement("div");
    newLine.className = "log-entry";
    newLine.innerText = msg;
    if (isError) newLine.style.color = "#ff5555";

    logContainer.appendChild(newLine);
    scrollToBottom();
}

