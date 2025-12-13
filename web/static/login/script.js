function login() {
    const username = document.getElementById('username').value;
    const password = document.getElementById('password').value;
    const errorMessage = document.getElementById('error-message');

    errorMessage.textContent = '';

    if (!username || !password) {
        errorMessage.textContent = 'Username and password are required.';
        return;
    }

    // Simulate a login request
    errorMessage.textContent = 'Authenticating...';
    setTimeout(() => {
        // In a real application, you would send a request to the server
        // to validate the credentials.
        // For this example, we'll just check if the credentials are not empty
        // and redirect to the main dashboard.
        
        // This is a placeholder for basic auth
        // A real implementation should use a secure authentication method
        // and handle the response from the server.
        
        // For now, we'll just redirect to the main page.
        window.location.href = '/';

    }, 1000);
}
