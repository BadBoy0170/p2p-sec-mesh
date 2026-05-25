export class NetworkGraph {
    constructor(containerId) {
        this.nodes = new vis.DataSet([]);
        this.edges = new vis.DataSet([]);
        this.container = document.getElementById(containerId);
        
        // Deep modern palette matching the reference image
        this.theme = {
            healthy: { background: '#2ecc71', border: '#27ae60', font: '#94a3b8' },
            attacked: { background: '#f39c12', border: '#e67e22', font: '#f39c12' },
            quarantined: { background: '#e74c3c', border: '#c0392b', font: '#e74c3c' },
            edgeNormal: 'rgba(142, 68, 173, 0.4)', // Purple-ish gradient feel
            edgeWarning: 'rgba(243, 156, 18, 0.6)'
        };

        this.initGraph();
    }

    initGraph() {
        const data = { nodes: this.nodes, edges: this.edges };
        const options = {
            layout: {
                // Strict hierarchical is removed to allow free 360-degree dragging
            },
            nodes: {
                shape: 'dot',
                size: 10,                 // Small circular avatars
                font: { 
                    size: 12, 
                    color: '#f8fafc', 
                    face: 'Inter, sans-serif',
                    align: 'left',        // Text to the right of the node
                    multi: 'html'
                },
                borderWidth: 2
            },
            edges: {
                width: 3,
                smooth: { 
                    type: 'cubicBezier',  // Smooth flowing curves
                    forceDirection: 'horizontal',
                    roundness: 0.6
                },
                arrows: {
                    to: { enabled: true, scaleFactor: 0.3, type: 'arrow' }
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
                dragNodes: true // Allow moving nodes freely
            }
        };

        this.network = new vis.Network(this.container, data, options);
    }

    setFilter(term) {
        this.filterTerm = (term || "").toLowerCase();
    }

    updateTopology(telemetryNodes) {
        // Apply filter
        const activeNodes = telemetryNodes.filter(t => !this.filterTerm || t.node_id.toLowerCase().includes(this.filterTerm));
        const validNodeIds = new Set(activeNodes.map(t => t.node_id));
        const newEdges = [];

        // In a true LR flow without physics, we assign arbitrary levels to simulate source/dest
        // We'll put 'api-gateway' at level 0, others at level 1 or 2 based on connections
        
        activeNodes.forEach(t => {
            const colors = this.theme[t.status] || this.theme.healthy;

            const cpu = t.cpu_pct ? t.cpu_pct.toFixed(1) : "0.0";
            const ram = t.ram_pct ? t.ram_pct.toFixed(1) : "0.0";
            
            // Assign level dynamically based on node name for better visual flow
            let nodeLevel = 1;
            if (t.node_id.includes("gateway")) nodeLevel = 0;
            if (t.node_id.includes("transaction")) nodeLevel = 2;

            this.nodes.update({
                id: t.node_id,
                label: `<b>${t.node_id}</b>\n${t.status.toUpperCase()} | CPU: ${cpu}% | RAM: ${ram}%`,
                level: nodeLevel,
                color: { 
                    background: colors.background, 
                    border: colors.border,
                    highlight: { background: '#ffffff', border: colors.background },
                }
            });

            // Build Edges
            if (t.status !== 'quarantined') {
                (t.peers || []).forEach(peer => {
                    const edgeId = [t.node_id, peer].sort().join('-');
                    const edgeColor = t.status === 'attacked' ? this.theme.edgeWarning : this.theme.edgeNormal;
                    
                    newEdges.push({
                        id: edgeId,
                        from: t.node_id,
                        to: peer,
                        color: { color: edgeColor, highlight: '#ffffff' }
                    });
                });
            }
        });

        // Cleanup stale nodes
        this.nodes.getIds().forEach(id => {
            if (!validNodeIds.has(id)) this.nodes.remove(id);
        });

        // Upsert and cleanup edges
        const validEdgeIds = new Set(newEdges.map(e => e.id));
        newEdges.forEach(e => this.edges.update(e));
        
        this.edges.getIds().forEach(id => {
            if (!validEdgeIds.has(id)) this.edges.remove(id);
        });
    }
}
