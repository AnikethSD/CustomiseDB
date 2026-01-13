const API_URL = 'http://localhost:8080';

// == STATE ==
let state = {
  nodes: [],
  mode: 'sync',
  stats: {}, // Map address -> stat object
  config: { replicas: 20 },
  selectedNode: null // Address of selected node
};

// == CONFIG ==
const COLOR_PRIMARY = '#00f0ff';
const COLOR_NODE_DEFAULT = '#4c5c75';
const COLOR_NODE_ACTIVE = '#00ff9d';
const COLOR_NODE_SELECTED = '#ff0055'; // Highlight color
const R_RING = 200;
const R_NODE = 25;

// == SETUP ==
const width = document.getElementById('viewport').clientWidth;
const height = document.getElementById('viewport').clientHeight;

const svg = d3.select('#d3-container')
  .append('svg')
  .attr('width', width)
  .attr('height', height);

const gRing = svg.append('g').attr('transform', `translate(${width / 2},${height / 2})`);
const gNodes = svg.append('g').attr('transform', `translate(${width / 2},${height / 2})`);
const gLinks = svg.append('g').attr('transform', `translate(${width / 2},${height / 2})`);

// Particle Canvas
const canvas = document.getElementById('particle-canvas');
canvas.width = width;
canvas.height = height;
const ctx = canvas.getContext('2d');
let particles = [];

// == D3 VISUALIZATION ==
function drawViz() {
  // 1. Draw Ring
  gRing.selectAll('*').remove();
  gRing.append('circle')
    .attr('r', R_RING)
    .attr('fill', 'none')
    .attr('stroke', 'rgba(60, 100, 255, 0.2)')
    .attr('stroke-width', 2);

  const nodeCount = state.nodes.length;
  const angleStep = (2 * Math.PI) / (nodeCount || 1);

  // Prepare Node Data with positions
  const nodesData = state.nodes.map((addr, i) => {
    const angle = i * angleStep - Math.PI / 2; // Start at top
    return {
      id: addr,
      x: Math.cos(angle) * R_RING,
      y: Math.sin(angle) * R_RING,
      stat: state.stats[addr] || { key_count: 0, request_rate: 0, keys: [] }
    };
  });

  // Draw Links (Center to Nodes)
  const links = gLinks.selectAll('line').data(nodesData);
  links.enter().append('line')
    .merge(links)
    .transition().duration(500)
    .attr('x1', 0)
    .attr('y1', 0)
    .attr('x2', d => d.x)
    .attr('y2', d => d.y)
    .attr('stroke', 'rgba(60, 100, 255, 0.1)');
  links.exit().remove();

  // Draw Physical Nodes
  const nodes = gNodes.selectAll('g.node').data(nodesData, d => d.id);

  const nodesEnter = nodes.enter().append('g')
    .attr('class', 'node')
    .attr('cursor', 'pointer')
    .on('click', (e, d) => selectNode(d.id))
    .on('mouseover', function (e, d) {
      if (state.selectedNode !== d.id) {
        d3.select(this).select('circle.outer').attr('stroke', COLOR_PRIMARY);
      }
      showTooltip(e, d);
    })
    .on('mouseout', function (e, d) {
      if (state.selectedNode !== d.id) {
        d3.select(this).select('circle.outer').attr('stroke', 'none');
      }
      hideTooltip();
    });

  nodesEnter.append('circle')
    .attr('class', 'outer')
    .attr('r', R_NODE + 5)
    .attr('fill', 'none')
    .attr('stroke', 'none')
    .attr('stroke-width', 2);

  nodesEnter.append('circle')
    .attr('class', 'inner')
    .attr('r', R_NODE)
    .attr('fill', '#1a1a2e')
    .attr('stroke', COLOR_PRIMARY)
    .attr('stroke-width', 2);

  nodesEnter.append('text')
    .attr('class', 'label-port')
    .attr('dy', -5)
    .attr('text-anchor', 'middle')
    .attr('fill', '#fff')
    .attr('font-family', 'monospace')
    .attr('font-size', '10px')
    .text(d => d.id.split(':')[1]); // Port

  // Key Count Indicator inside node
  nodesEnter.append('text')
    .attr('class', 'label-keys')
    .attr('dy', 15)
    .attr('text-anchor', 'middle')
    .attr('fill', COLOR_PRIMARY)
    .attr('font-family', 'monospace')
    .attr('font-size', '9px')
    .text('0 keys');

  // Update positions & styles
  const nodesUpdate = nodesEnter.merge(nodes);

  nodesUpdate.transition().duration(500)
    .attr('transform', d => `translate(${d.x},${d.y})`);

  nodesUpdate.select('circle.inner')
    .attr('stroke', d => state.selectedNode === d.id ? COLOR_NODE_SELECTED : COLOR_PRIMARY)
    .attr('fill', d => state.selectedNode === d.id ? 'rgba(255, 0, 85, 0.2)' : '#1a1a2e');

  nodesUpdate.select('circle.outer')
    .attr('stroke', d => state.selectedNode === d.id ? COLOR_NODE_SELECTED : 'none');

  nodesUpdate.select('text.label-keys')
    .text(d => `${d.stat.key_count} keys`);

  nodes.exit().remove();
}

// == PARTICLES ==
class Particle {
  constructor(startX, startY, targetX, targetY, color) {
    this.x = startX;
    this.y = startY;
    this.targetX = targetX;
    this.targetY = targetY;
    this.color = color;
    this.progress = 0;
    this.speed = 0.02 + Math.random() * 0.02;
  }

  update() {
    this.progress += this.speed;
    if (this.progress >= 1) return false; // Dead

    const dx = this.targetX - this.x;
    const dy = this.targetY - this.y;
    this.currentX = this.x + dx * this.progress;
    this.currentY = this.y + dy * this.progress;
    return true;
  }

  draw(ctx) {
    ctx.beginPath();
    ctx.arc(this.currentX, this.currentY, 3, 0, Math.PI * 2);
    ctx.fillStyle = this.color;

    ctx.shadowBlur = 10;
    ctx.shadowColor = this.color;
    ctx.fill();
    ctx.shadowBlur = 0;
  }
}

function animateParticles() {
  ctx.clearRect(0, 0, width, height);
  particles = particles.filter(p => p.update());
  particles.forEach(p => p.draw(ctx));
  requestAnimationFrame(animateParticles);
}

function spawnRequest(targetPort) {
  const nodeCount = state.nodes.length;
  const angleStep = (2 * Math.PI) / (nodeCount || 1);

  // Find node index
  const idx = state.nodes.findIndex(n => n.includes(targetPort));
  // If not found (or load balancing), just pick random for effect if strict match fails
  const finalIdx = idx === -1 ? Math.floor(Math.random() * nodeCount) : idx;

  if (finalIdx === -1) return;

  const angle = finalIdx * angleStep - Math.PI / 2;
  const targetX = width / 2 + Math.cos(angle) * R_RING;
  const targetY = height / 2 + Math.sin(angle) * R_RING;

  particles.push(new Particle(width / 2, height / 2, targetX, targetY, '#fff'));
}

// == LOGIC ==
async function fetchStatus() {
  try {
    const res = await fetch(`${API_URL}/status`);
    const data = await res.json();

    const statsMap = {};
    data.stats.forEach(s => statsMap[s.address] = s);

    // Persist selected node if it still exists
    let newSelected = state.selectedNode;
    if (newSelected && !data.nodes.includes(newSelected)) {
      newSelected = null;
    }

    state = {
      nodes: data.nodes,
      mode: data.mode,
      stats: statsMap,
      config: data.config,
      selectedNode: newSelected
    };

    updateHUD();
    updateInspector();
    drawViz();
  } catch (e) {
    console.error("Poll error", e);
  }
}

function updateHUD() {
  // Mode
  document.getElementById('mode-display').innerText = state.mode.toUpperCase();

  // Toggles
  document.querySelectorAll('#mode-toggles button').forEach(btn => {
    if (btn.dataset.mode === state.mode) btn.classList.add('active');
    else btn.classList.remove('active');
  });

  // CAP
  const capInfo = document.getElementById('cap-info');
  const capLed = document.getElementById('cap-led');
  if (state.mode === 'async') {
    capInfo.innerHTML = `<strong>AP MODE</strong>: High Availability. Consistency eventual.`;
    capLed.style.background = 'var(--warning)';
    capLed.style.boxShadow = '0 0 8px var(--warning)';
  } else {
    capInfo.innerHTML = `<strong>CP MODE</strong>: Strict Consistency. Writes may fail if partitions occur.`;
    capLed.style.background = 'var(--success)';
    capLed.style.boxShadow = '0 0 8px var(--success)';
  }

  // Metrics
  document.getElementById('metric-nodes').innerText = state.nodes.length;
  const totalKeys = Object.values(state.stats).reduce((acc, s) => acc + s.key_count, 0);
  document.getElementById('metric-keys').innerText = totalKeys;
}

function selectNode(addr) {
  state.selectedNode = addr;
  updateInspector();
  drawViz(); // Force redraw for selection highlight
}

function updateInspector() {
  const container = document.getElementById('inspector-content');
  if (!state.selectedNode) {
    container.innerHTML = '<div class="inspector-empty">Click a node to view its keys</div>';
    return;
  }

  const s = state.stats[state.selectedNode];
  if (!s) return;

  const keyBadges = (s.keys && s.keys.length > 0)
    ? s.keys.map(k => `<span class="key-badge">${k}</span>`).join('')
    : '<span class="text-dim" style="font-size:0.7rem; padding:5px;">No keys stored</span>';

  container.innerHTML = `
        <div class="node-detail-header">
            <span class="node-detail-title">Worker ${state.selectedNode.split(':')[1]}</span>
            <span style="color:var(--text-dim); font-size:0.7rem;">${s.request_rate} req/s</span>
        </div>
        <div class="key-list">
            ${keyBadges}
        </div>
    `;
}

function log(msg, type = 'info') {
  const stream = document.getElementById('log-stream');
  const div = document.createElement('div');
  div.className = `log-line`;
  const time = new Date().toLocaleTimeString('en-US', { hour12: false });
  div.innerHTML = `<span class="ts">${time}</span> <span class="${type}">${type.toUpperCase()}</span> ${msg}`;
  stream.appendChild(div);
  stream.scrollTop = stream.scrollHeight;
}

function showTooltip(e, d) {
  const tt = document.getElementById('tooltip');
  tt.classList.remove('hidden');
  tt.style.left = (e.pageX + 15) + 'px';
  tt.style.top = (e.pageY + 15) + 'px';
  tt.innerHTML = `
        <strong>${d.id}</strong><br>
        Keys: ${d.stat.key_count}<br>
        Load: ${d.stat.request_rate}/s
    `;
}

function hideTooltip() {
  document.getElementById('tooltip').classList.add('hidden');
}

// == INTERACTION ==
document.querySelectorAll('#mode-toggles button').forEach(btn => {
  btn.addEventListener('click', async () => {
    const mode = btn.dataset.mode;
    await fetch(`${API_URL}/config`, {
      method: 'POST', body: JSON.stringify({ mode })
    });
    log(`Switched replication mode to ${mode}`, 'sys');
  });
});

// Put Handler
document.getElementById('btn-put').addEventListener('click', async () => {
  const keyInput = document.getElementById('put-key');
  const valInput = document.getElementById('put-val');
  const key = keyInput.value.trim();
  const val = valInput.value.trim();

  if (!key || !val) return;

  try {
    await fetch(`${API_URL}/put?key=${key}&value=${val}`);
    log(`PUT ${key} = ${val}`, 'success');
    keyInput.value = '';
    valInput.value = '';
    spawnRequest('???'); // We don't know the exact target node here easily without shared hash logic
  } catch (e) {
    log(`PUT Failed: ${e}`, 'error');
  }
});

// Get Handler
document.getElementById('btn-get').addEventListener('click', async () => {
  const keyInput = document.getElementById('get-key');
  const key = keyInput.value.trim();

  if (!key) return;

  try {
    const res = await fetch(`${API_URL}/get?key=${key}`);
    if (res.ok) {
      const val = await res.text();
      log(`GET ${key} -> ${val}`, 'success');
    } else {
      log(`GET ${key} -> NOT FOUND`, 'warning');
    }
  } catch (e) {
    log(`GET Failed: ${e}`, 'error');
  }
});


// Init
animateParticles();
setInterval(fetchStatus, 1000);
fetchStatus();
log('Dashboard initialized. Connecting to Master...', 'sys');
