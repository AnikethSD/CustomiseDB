const API_URL = 'http://localhost:8080';

// State
let state = {
    nodes: [],
    mode: 'sync',
    stats: [],
    config: { replicas: 20 }
};

// Colors for Nodes (Consistent coloring based on port)
const NODE_COLORS = [
    '#3b82f6', '#8b5cf6', '#10b981', '#f59e0b', '#ef4444',
    '#ec4899', '#6366f1', '#14b8a6', '#f97316', '#84cc16'
];

function getNodeColor(addr) {
    if (!addr) return '#64748b';
    const port = parseInt(addr.split(':')[1] || '0');
    return NODE_COLORS[port % NODE_COLORS.length];
}

// Canvas Setup
const canvas = document.getElementById('ring-canvas');
const ctx = canvas.getContext('2d');

function resizeCanvas() {
    const container = canvas.parentElement;
    canvas.width = container.clientWidth;
    canvas.height = container.clientHeight;
}
window.addEventListener('resize', resizeCanvas);
resizeCanvas();

// CRC32 Implementation for Ring Viz (Matching Go's logic roughly for display)
function crc32(str) {
    // Simple mock hash for visual distribution if actual values aren't sent
    // For true accuracy, backend should send ring positions, but we'll simulate
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        hash = ((hash << 5) - hash) + str.charCodeAt(i);
        hash |= 0;
    }
    return Math.abs(hash);
}

// Drawing Logic
function drawRing() {
    const { width, height } = canvas;
    const centerX = width / 2;
    const centerY = height / 2;
    const radius = Math.min(width, height) / 2 - 60;

    ctx.clearRect(0, 0, width, height);

    // Draw Ring Circle
    ctx.beginPath();
    ctx.arc(centerX, centerY, radius, 0, Math.PI * 2);
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)';
    ctx.lineWidth = 2;
    ctx.stroke();

    // Draw Virtual Nodes
    // Since we don't stream the full ring (it's big), we simulate the distribution logic
    // based on the known active nodes and replica count.

    // Total space is 2^32, but we map to 0..2PI
    // We'll generate the same deterministic virtual nodes as Go

    // Note: In a real app we'd fetch the exact ring keys from Backend.
    // Here we simulate the distribution visually.

    state.nodes.forEach(nodeAddr => {
        const color = getNodeColor(nodeAddr);
        for (let i = 0; i < state.config.replicas; i++) {
            // Re-implement simplified hash visual logic
            // Use a simple seeded random-ish placement based on Node+Index to look consistent
            const seed = nodeAddr + i;
            const pseudoHash = crc32(seed) % 1000;
            const angle = (pseudoHash / 1000) * Math.PI * 2;

            const x = centerX + Math.cos(angle) * radius;
            const y = centerY + Math.sin(angle) * radius;

            ctx.beginPath();
            ctx.arc(x, y, 4, 0, Math.PI * 2);
            ctx.fillStyle = color;
            ctx.fill();
        }
    });

    // Draw Physical Nodes (Hubs)
    state.nodes.forEach((nodeAddr, idx) => {
        const color = getNodeColor(nodeAddr);
        // Place them in a smaller concentric circle for Legend
        const innerRadius = radius * 0.6;
        const angle = (idx / state.nodes.length) * Math.PI * 2;

        const x = centerX + Math.cos(angle) * innerRadius;
        const y = centerY + Math.sin(angle) * innerRadius;

        // Connection lines to ring (just for effect)
        ctx.beginPath();
        ctx.moveTo(centerX, centerY);
        ctx.lineTo(x, y);
        ctx.strokeStyle = `rgba(255,255,255,0.05)`;
        ctx.stroke();

        // Node Circle
        ctx.beginPath();
        ctx.arc(x, y, 15, 0, Math.PI * 2);
        ctx.fillStyle = color;
        ctx.shadowBlur = 15;
        ctx.shadowColor = color;
        ctx.fill();
        ctx.shadowBlur = 0; // reset

        // Label
        ctx.fillStyle = '#fff';
        ctx.font = '10px Inter';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(nodeAddr.split(':')[1], x, y);

        ctx.fillStyle = 'rgba(255,255,255,0.5)';
        ctx.fillText("Node", x, y + 25);
    });
}

// Logic
async function fetchStatus() {
    try {
        const res = await fetch(`${API_URL}/status`);
        const data = await res.json();

        // Diff check to avoid redraw if nothing changed? 
        // For now just update.
        state = data;

        updateUI();
        drawRing(); // Re-render viz
    } catch (e) {
        console.error("Polling error", e);
    }
}

function updateUI() {
    // 1. Update Badge
    const badge = document.getElementById('cap-badge');
    const type = document.getElementById('cap-type');
    const dot = document.querySelector('.status-dot');

    if (state.mode === 'async') {
        type.innerText = 'AP'; // Available / Partition Tolerant
        type.style.color = 'var(--warning)';
        dot.style.background = 'var(--warning)';
        dot.style.boxShadow = '0 0 5px var(--warning)';
    } else {
        type.innerText = 'CP'; // Consistent / Partition Tolerant
        type.style.color = 'var(--success)';
        dot.style.background = 'var(--success)';
        dot.style.boxShadow = '0 0 5px var(--success)';
    }

    // 2. Metrics
    document.getElementById('worker-count').innerText = state.nodes.length;

    const totalKeys = state.stats.reduce((acc, s) => acc + s.key_count, 0);
    document.getElementById('total-keys').innerText = totalKeys;

    // 3. Worker List
    const list = document.getElementById('worker-list');
    list.innerHTML = state.stats.map(s => {
        const keyList = s.keys && s.keys.length > 0
            ? s.keys.map(k => `<span class="key-badge">${k}</span>`).join('')
            : '<span class="text-dim">Empty</span>';

        return `
        <div class="worker-item">
            <div class="worker-header">
                <span style="color:${getNodeColor(s.address)}">Worker ${s.address.split(':')[1]}</span>
                <span>${s.request_rate} req/s</span>
            </div>
            <div class="worker-keys">
                ${keyList}
            </div>
        </div>
    `}).join('');

    // 4. Update Select if changed externally
    const select = document.getElementById('mode-select');
    if (select.value !== state.mode) {
        select.value = state.mode;
    }
}

// Interaction
document.getElementById('mode-select').addEventListener('change', async (e) => {
    const mode = e.target.value;
    try {
        await fetch(`${API_URL}/config`, {
            method: 'POST',
            body: JSON.stringify({ mode })
        });
        logConsole(`Mode switched to ${mode.toUpperCase()}`);
    } catch (e) {
        logConsole(`Error switching mode: ${e}`);
    }
});

document.getElementById('btn-put').addEventListener('click', async () => {
    const key = document.getElementById('key-input').value;
    const val = document.getElementById('val-input').value;
    if (!key || !val) return;

    try {
        const res = await fetch(`${API_URL}/put?key=${key}&value=${val}`);
        const text = await res.text();
        logConsole(`PUT ${key}: ${text}`);
    } catch (e) {
        logConsole(`PUT Failed: ${e}`);
    }
});

document.getElementById('btn-get').addEventListener('click', async () => {
    const key = document.getElementById('key-input').value;
    if (!key) return;

    try {
        const res = await fetch(`${API_URL}/get?key=${key}`);
        if (res.ok) {
            const val = await res.text();
            logConsole(`GET ${key} -> ${val}`);
        } else {
            logConsole(`GET ${key} -> Not Found`);
        }
    } catch (e) {
        logConsole(`GET Failed: ${e}`);
    }
});

function logConsole(msg) {
    const el = document.getElementById('console-output');
    el.innerText = `> ${msg}`;
    el.classList.remove('placeholder');
}

// Loop
setInterval(fetchStatus, 1000); // 1s polling
fetchStatus();
