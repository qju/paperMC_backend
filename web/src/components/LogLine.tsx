import Anser from 'anser';

export function LogLine({ content }: { content: string }) {
    // 1. Determine the "Base Color" of the line
    // This color is used for any text that doesn't have its own ANSI color code.
    let baseColorClass = "text-gray-300"; // <--- CHANGE THIS for your default color

    if (content.includes("ERROR") || content.includes("Exception")) {
        baseColorClass = "text-red-400";
    } else if (content.includes("WARN")) {
        baseColorClass = "text-mc-gold";
    } else if (content.includes("INFO")) {
        baseColorClass = "text-blue-300"; // Your preferred INFO color
    }

    // 2. Parse the ANSI codes
    const chunks = Anser.ansiToJson(content, {
        use_classes: false,
        json: true
    });

    return (
        // 3. Apply the base color to the wrapper DIV
        <div className={`font-mono text-[13px] leading-tight break-words whitespace-pre-wrap ${baseColorClass}`}>
            {chunks.map((chunk, i) => (
                <span
                    key={i}
                    style={{
                        // If the chunk has a specific color, use it. Otherwise, inherit from parent.
                        color: chunk.fg ? `rgb(${chunk.fg})` : undefined,
                        backgroundColor: chunk.bg ? `rgb(${chunk.bg})` : undefined,
                        // Handle bold text if needed
                        fontWeight: chunk.decoration === 'bold' ? 'bold' : 'normal'
                    }}
                >
                    {chunk.content}
                </span>
            ))}
        </div>
    );
}
