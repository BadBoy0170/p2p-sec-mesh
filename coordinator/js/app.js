import { ApiService } from './ApiService.js';
import { NetworkGraph } from './NetworkGraph.js';

class DashboardController {
    constructor() {
        this.api = new ApiService('/api/topology');
        this.graph = new NetworkGraph('network-container');
        this.pollInterval = 5000; // Poll every 5s instead of 2s to save CPU
        
        // DOM Elements
        this.ui = {
            total: document.getElementById('stat-total'),
            healthy: document.getElementById('stat-healthy'),
            threats: document.getElementById('stat-threats'),
            quarantined: document.getElementById('stat-quarantined'),
            syncIndicator: document.getElementById('sync-indicator'),
            searchInput: document.getElementById('search-input')
        };

        if (this.ui.searchInput) {
            this.ui.searchInput.addEventListener('input', (e) => {
                this.graph.setFilter(e.target.value);
                this.syncState();
            });
        }
    }

    updateMetrics(nodes) {
        let stats = { healthy: 0, attacked: 0, quarantined: 0 };
        
        nodes.forEach(node => {
            if (stats[node.status] !== undefined) {
                stats[node.status]++;
            }
        });

        this.ui.total.innerText = nodes.length;
        this.ui.healthy.innerText = stats.healthy;
        this.ui.threats.innerText = stats.attacked;
        this.ui.quarantined.innerText = stats.quarantined;
    }

    async syncState() {
        try {
            const data = await this.api.getTopology();
            this.graph.updateTopology(data.nodes || []);
            this.updateMetrics(data.nodes || []);
            
            this.ui.syncIndicator.classList.remove('error');
        } catch (error) {
            this.ui.syncIndicator.classList.add('error');
        }
    }

    start() {
        this.syncState();
        setInterval(() => this.syncState(), this.pollInterval);
    }
}

// Bootstrap Application
document.addEventListener('DOMContentLoaded', () => {
    const app = new DashboardController();
    app.start();
});
