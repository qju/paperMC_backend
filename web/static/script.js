function checkAuth() {
    fetch('/status', {
        headers: {
            'Authorization': 'Bearer ' + localStorage.getItem('token')
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

// Run immediately when page loads
document.addEventListener("DOMContentLoaded", () => {
    checkAuth();
    // ... rest of your init code ...
});

const cmdInput = document.getElementById('cmd-input');
const statusBadge = document.getElementById('status-badge');

function updateStatus() {
    fetch('/status')
        .then(response => response.json())
        .then(data => {
            // data.status comes from your GO struct "status"
            statusBadge.innerText = data.status;
            statusBadge.className = "status-badge"
            // Visual Polish: Change class based on status
            if (data.status === "Running") {
                statusBadge.classList.add('running');
            } else {
                statusBadge.classList.add('stopped');
            }
        })
        .catch(err => {
            console.error("API Error: ", err);
            statusBadge.classList.add('stopped');
            statusBadge.innerText = "Offline (Backend down)"
        });
}

function startServer() {
    fetch('/start', {method: 'POST'})
        .then(updateStatus);
}

function stopServer() {
    fetch('/stop', {method: 'POST'})
        .then(updateStatus);
}

function sendCommand() {
    const cmd = cmdInput.value;
    if (!cmd) return;

    fetch('/command', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({command: cmd})
    }).then(resp => {
        if(resp.ok) {
            cmdInput.value = ''; 
        } else {
            addLocalLog("System: Command failed to send.", true);
        }
    });
}

function openModalWhitelist() {
    document.getElementById("modal-overlay").classList.remove('hidden')
    document.getElementById("modal-input").focus() //clear it
}

function openModalConfig() {
    document.getElementById("config-modal-overlay").classList.remove('hidden')
}
function openModalUpdate() {
    document.getElementById("update-modal-overlay").classList.remove('hidden')
}

function closeModal() {
    document.getElementById("modal-overlay").classList.add('hidden')
    document.getElementById("modal-input").value = '' //clear it
}
function closeModalUpdate() {
    document.getElementById("update-modal-overlay").classList.add('hidden')
}

function whiteList() {
    const wlInput = document.getElementById('modal-input');
    const cmd = wlInput.value;

    if (!cmd) return;

    fetch('/whitelist_add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({command: cmd})
    }).then(resp => {
        if(resp.ok) {
            wlInput.value = ''; 
            addLocalLog(`[System] Whitelist request sent for: ${cmd}`);
        } else {
            addLocalLog("System: Command failed to send.", true);
        }
    });
}
function openModalConfig() {
    document.getElementById("config-modal-overlay").classList.remove('hidden');
    loadConfigData(); // Trigger the fetch
}
function closeModalConfig() {
    document.getElementById("config-modal-overlay").classList.add('hidden')
}

function loadConfigData() {
    const container = document.getElementById('config-container');
    container.innerHTML = '<div style="text-align:center;">Loading...</div>';

    fetch('/config')
        .then(res => res.json())
        .then(data => {
            container.innerHTML = ''; // Clear loading text
            
            // Loop through map keys
            // Object.keys(data).sort() ensures we display them alphabetically
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
                label.style.width = "100%" 

                const input = document.createElement('input');
                input.type = 'text';
                input.value = data[key];
                
                // FIX: Add 'modal-input' to get the CSS styling!
                input.className = 'config-input modal-input'; 
                input.dataset.key = key;
                
                // REMOVED: All the input.style lines. 
                // Let CSS handle the look.

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

    fetch('/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
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

// option close modal on "Escape" key
document.addEventListener('keydown', function(event) {
    if (event.key === "Escape") {
        closeModal();
        closeModalConfig();
    }
});

const eventSources = new EventSource("/logs");
const logContainer = document.getElementById("log-container");
//Listening for message
eventSources.onmessage = function(event) {
    console.log("New Log: ", event.data)
    
    const newLine = document.createElement("div");
    
    newLine.innerText = event.data

    if (event.data.includes("ERROR") || event.data.includes("Exception")) {
        newLine.style.color = "red";
    }

    logContainer.appendChild(newLine);

    logContainer.scrollTop = logContainer.scrollHeight;
};

eventSources.onerror = function() {
    eventSources.close();
};

    // --- Helpers ---

function handleEnter(event) {
    if (event.key === 'Enter') {
        sendCommand();
    }
}
function scrollToBottom() {
    logContainer.scrollTop = logContainer.scrollHeight;
}

function addLocalLog(msg, isError = false) {
    const newLine = document.createElement("div");
    newLine.className = "log-entry";
    newLine.innerText = msg;
    if (isError) newLine.classList.add("log-error");
    
    logContainer.appendChild(newLine);
    scrollToBottom();
}

setInterval(updateStatus, 5000);
updateStatus();
