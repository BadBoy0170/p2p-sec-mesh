export class NetworkGraph {
    constructor(containerId) {
        this.nodes = new vis.DataSet([]);
        this.edges = new vis.DataSet([]);
        this.container = document.getElementById(containerId);
        this.filterTerm = '';

        // Muted semantic palette
        this.theme = {
            healthy:    { background: '#1a2e23', border: '#3d9970' },
            attacked:   { background: '#2c2210', border: '#b07d2f' },
            quarantined:{ background: '#2a1010', border: '#922b21' },
            edgeNormal:  'rgba(255,255,255,0.07)',
            edgeWarning: 'rgba(176,125,47,0.35)'
        };

        // Glow pulse animation state
        this._glowNodes  = new Map(); // nodeId → { color, phase }
        this._animFrame  = null;

        this.initGraph();
        this._startGlowLoop();
    }

    initGraph() {
        const data = { nodes: this.nodes, edges: this.edges };
        const options = {
            layout: {},
            nodes: {
                shape: 'dot',
                size: 7,
                font: {
                    size: 11,
                    color: '#555555',
                    face: 'ui-monospace, Menlo, Monaco, Consolas, monospace',
                    align: 'left',
                    multi: false
                },
                borderWidth: 1
            },
            edges: {
                width: 1,
                smooth: false,
                arrows: {
                    to: { enabled: true, scaleFactor: 0.25, type: 'arrow' }
                }
            },
            physics: {
                solver: 'forceAtlas2Based',
                forceAtlas2Based: {
                    gravitationalConstant: -120,
                    springLength: 250,
                    centralGravity: 0.01
                }
            },
            interaction: {
                hover: true,
                tooltipDelay: 200,
                dragNodes: true
            }
        };

        this.network = new vis.Network(this.container, data, options);

        // Stop physics after stabilisation for performance
        this.network.on('stabilizationIterationsDone', () => {
            this.network.setOptions({ physics: false });
        });

        // Expose canvas for the glow overlay
        this.network.on('afterDrawing', (ctx) => {
            this._drawGlowOverlay(ctx);
        });
    }

    // ── Glow / Pulse loop ──────────────────────────────────────────────────────

    /**
     * Runs a rAF loop that advances the glow phase for each tracked node and
     * triggers a canvas redraw.  Only active when there are nodes to animate.
     */
    _startGlowLoop() {
        const FPS_TARGET = 30;
        const FRAME_MS   = 1000 / FPS_TARGET;
        let lastTime = 0;

        const loop = (ts) => {
            this._animFrame = requestAnimationFrame(loop);
            if (ts - lastTime < FRAME_MS) return;
            lastTime = ts;

            if (this._glowNodes.size > 0 && this.network) {
                // Advance phase
                this._glowNodes.forEach((state) => {
                    state.phase = (state.phase + 0.04) % (Math.PI * 2);
                });
                this.network.redraw();
            }
        };
        this._animFrame = requestAnimationFrame(loop);
    }

    /**
     * Canvas overlay called by vis.js afterDrawing.
     * Draws a soft radial glow ring around anomaly/quarantined nodes.
     */
    _drawGlowOverlay(ctx) {
        if (this._glowNodes.size === 0) return;

        this._glowNodes.forEach((state, nodeId) => {
            const pos = this.network.getPositions([nodeId])[nodeId];
            if (!pos) return;

            const canvasPos = this.network.canvasToDOM(pos);
            const scale     = this.network.getScale();
            const baseR     = 7 * scale;            // node radius in screen px
            const pulse     = 0.5 + 0.5 * Math.sin(state.phase); // 0..1
            const glowR     = baseR + 4 + 6 * pulse;  // expanding ring

            const gradient = ctx.createRadialGradient(
                pos.x, pos.y, baseR * 0.6,
                pos.x, pos.y, glowR * 1.4
            );

            // Color: amber for attacked, red for quarantined
            const [r, g, b] = state.rgb;
            const alpha = 0.12 + 0.20 * pulse;    // gentle fade
            gradient.addColorStop(0,   `rgba(${r},${g},${b},${alpha.toFixed(2)})`);
            gradient.addColorStop(0.5, `rgba(${r},${g},${b},${(alpha * 0.5).toFixed(2)})`);
            gradient.addColorStop(1,   `rgba(${r},${g},${b},0)`);

            ctx.save();
            ctx.beginPath();
            ctx.arc(pos.x, pos.y, glowR * 1.4, 0, Math.PI * 2);
            ctx.fillStyle = gradient;
            ctx.fill();
            ctx.restore();
        });
    }

    // ── Filter ─────────────────────────────────────────────────────────────────

    setFilter(term) {
        this.filterTerm = (term || '').toLowerCase();
    }

    // ── Topology update ────────────────────────────────────────────────────────

    updateTopology(telemetryNodes) {
        const activeNodes = telemetryNodes.filter(
            t => !this.filterTerm || t.node_id.toLowerCase().includes(this.filterTerm)
        );
        const validNodeIds = new Set(activeNodes.map(t => t.node_id));

        const newNodesToUpdate = [];
        const newEdgesToUpdate = [];

        // Reset glow tracking; will be rebuilt below
        const nextGlow = new Map();

        activeNodes.forEach(t => {
            const colors = this.theme[t.status] || this.theme.healthy;
            const cpu = t.cpu_pct ? t.cpu_pct.toFixed(1) : '0.0';
            const ram = t.ram_pct ? t.ram_pct.toFixed(1) : '0.0';

            let nodeLevel = 1;
            if (t.node_id.includes('gateway'))    nodeLevel = 0;
            if (t.node_id.includes('transaction')) nodeLevel = 2;

            newNodesToUpdate.push({
                id: t.node_id,
                label: t.node_id,
                title: `${t.node_id}\nStatus: ${t.status}\nCPU: ${cpu}%  RAM: ${ram}%`,
                level: nodeLevel,
                color: {
                    background: colors.background,
                    border:     colors.border,
                    highlight:  { background: colors.background, border: colors.border }
                }
            });

            // Register glow for anomaly and quarantined nodes
            if (t.status === 'attacked') {
                const existing = this._glowNodes.get(t.node_id);
                nextGlow.set(t.node_id, {
                    rgb:   [176, 125, 47],      // amber
                    phase: existing ? existing.phase : Math.random() * Math.PI * 2
                });
            } else if (t.status === 'quarantined') {
                const existing = this._glowNodes.get(t.node_id);
                nextGlow.set(t.node_id, {
                    rgb:   [146, 43, 33],       // dark red
                    phase: existing ? existing.phase : Math.random() * Math.PI * 2
                });
            }

            // Build Edges
            if (t.status !== 'quarantined') {
                (t.peers || []).forEach(peer => {
                    const edgeId    = [t.node_id, peer].sort().join('-');
                    const edgeColor = t.status === 'attacked'
                        ? this.theme.edgeWarning
                        : this.theme.edgeNormal;
                    newEdgesToUpdate.push({
                        id:    edgeId,
                        from:  t.node_id,
                        to:    peer,
                        color: { color: edgeColor, highlight: '#ffffff' }
                    });
                });
            }
        });

        // Swap glow map
        this._glowNodes = nextGlow;

        // Batch update vis.js datasets
        this.nodes.update(newNodesToUpdate);
        this.edges.update(newEdgesToUpdate);

        // Cleanup stale nodes
        const nodesToRemove = this.nodes.getIds().filter(id => !validNodeIds.has(id));
        if (nodesToRemove.length > 0) this.nodes.remove(nodesToRemove);

        // Cleanup stale edges
        const validEdgeIds = new Set(newEdgesToUpdate.map(e => e.id));
        const edgesToRemove = this.edges.getIds().filter(id => !validEdgeIds.has(id));
        if (edgesToRemove.length > 0) this.edges.remove(edgesToRemove);
    }
}
