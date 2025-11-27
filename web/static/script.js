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
