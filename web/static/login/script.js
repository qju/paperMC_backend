async function login() {
    const usernameInput = document.getElementById('username');
    const passwordInput = document.getElementById('password');
    const errorMessage = document.getElementById('error-message');

    errorMessage.textContent = 'Authenticating...';
    errorMessage.style.color = "#00ff00"; // Green for loading

    try {
        const response = await fetch('/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                username: usernameInput.value,
                password: passwordInput.value
            })
        });

        if (!response.ok) {
            throw new Error("Invalid Credentials");
        }

        const data = await response.json();

        // 1. SAVE THE TOKEN
        localStorage.setItem('token', data.token);

        // 2. Redirect to Dashboard
        window.location.href = '/';

    } catch (err) {
        console.error(err);
        errorMessage.style.color = "#ff0000";
        errorMessage.textContent = 'Login Failed: ' + err.message;
    }
}
