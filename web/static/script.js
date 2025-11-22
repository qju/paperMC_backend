function updateStatus() {
    fetch('/status')
        .then(response => response.json())
        .then(data => {
            // data.status comes from your GO struct "status"
            const statusSpan = document.getElementById('current-status');
            statusSpan.innerText = data.status;

            // Visual Polish: Change class based on status
            if (data.status === "Running") {
                statusSpan.classList.remove('stopped');
                statusSpan.classList.add('running');
            } else {
                statusSpan.classList.remove('running');
                statusSpan.classList.add('stopped');
            }
        })
        .catch(err => {
            console.error("API Error: ", err);
            document.getElementById("current-status").innerText = "Offline (Backend down)";
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

function sendCommmand() {
    const cmd = document.getElementById('cmd-input').value;
    fetch('/command', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({command: cmd})
    });
}


setInterval(updateStatus, 5000);
updateStatus();
