/** @type {import('tailwindcss').Config} */
export default {
    content: [
        "./index.html",
        "./src/**/*.{js,ts,jsx,tsx}",
    ],
    theme: {
        extend: {
            colors: {
                mc: {
                    bg: '#111111',       // Deep Bedrock
                    surface: '#1a1a1a',  // Surface Block
                    green: '#55aa55',    // Success
                    red: '#aa0000',      // Error
                    gold: '#ffaa00',     // Warning
                    diamond: '#55ffff',  // Accent
                    gray: '#8b8b8b',     // Stone
                }
            },
            fontFamily: {
                pixel: ['"VT323"', 'monospace'],
                mono: ['"JetBrains Mono"', '"Fira Code"', 'monospace'],
            },
            backgroundImage: {
                'dirt-pattern': "repeating-linear-gradient(45deg, #1a1a1a 25%, transparent 25%, transparent 75%, #1a1a1a 75%, #1a1a1a), repeating-linear-gradient(45deg, #1a1a1a 25%, #111111 25%, #111111 75%, #1a1a1a 75%, #1a1a1a)",
            }
        },
    },
    plugins: [],
}

