import { ApiService } from './ApiService.js';
import { NetworkGraph } from './NetworkGraph.js';

class DashboardController {
    constructor() {
        this.api = new ApiService('/api/topology');
        this.graph = new NetworkGraph('network-container');
        this.pollInterval = 5000;
        this.reportPollInterval = 4000;

        // DOM Elements
        this.ui = {
            total: document.getElementById('stat-total'),
            healthy: document.getElementById('stat-healthy'),
            threats: document.getElementById('stat-threats'),
            quarantined: document.getElementById('stat-quarantined'),
            syncIndicator: document.getElementById('sync-indicator'),
            searchInput: document.getElementById('search-input'),
            reportFeed: document.getElementById('report-feed'),
            reportCount: document.getElementById('report-count'),
        };

        this._seenReports = new Set();

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
            if (stats[node.status] !== undefined) stats[node.status]++;
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

    async syncReports() {
        try {
            const res = await fetch('/api/report');
            if (!res.ok) return;
            const data = await res.json();
            const events = (data.events || []).slice().reverse(); // newest first

            let newCount = 0;
            events.forEach(ev => {
                const key = `${ev.node_id}-${ev.reported_at}`;
                if (!this._seenReports.has(key)) {
                    this._seenReports.add(key);
                    newCount++;
                    this._prependReportEvent(ev);
                }
            });

            this.ui.reportCount.innerText = this._seenReports.size;
        } catch (_) { }
    }

    _prependReportEvent(ev) {
        // Remove empty placeholder
        const placeholder = this.ui.reportFeed.querySelector('.report-empty');
        if (placeholder) placeholder.remove();

        const score = typeof ev.score === 'number' ? ev.score.toFixed(1) : '?';
        const method = (ev.method || 'rule-based').toLowerCase();
        const decision = (ev.decision || 'monitor').toLowerCase();
        const nodeShort = (ev.node_id || 'unknown').split('').slice(0, 18).join('');
        const srcIP = ev.source_ip || '—';
        const time = ev.reported_at
            ? new Date(ev.reported_at).toLocaleTimeString('en-US', { hour12: false })
            : '';

        const methodBadgeClass = method === 'ai' ? 'ai' : 'rule';
        const methodLabel = method === 'ai' ? 'AI' : 'RULE';
        const decisionBadgeClass = decision === 'quarantine' ? 'quarantine' : 'monitor';

        const div = document.createElement('div');
        div.className = 'report-event';
        div.innerHTML = `
            <div class="report-event-node">${nodeShort}</div>
            <div class="report-event-meta">
                <span>score ${score}/10</span>
                <span>src ${srcIP}</span>
                <span class="report-badge ${decisionBadgeClass}">${decision}</span>
                <span class="report-badge ${methodBadgeClass}">${methodLabel}</span>
                ${time ? `<span>${time}</span>` : ''}
            </div>
        `;

        // Prepend: insert before the first child
        this.ui.reportFeed.insertBefore(div, this.ui.reportFeed.firstChild);

        // Cap the feed at 50 visible events
        const items = this.ui.reportFeed.querySelectorAll('.report-event');
        if (items.length > 50) {
            items[items.length - 1].remove();
        }
    }

    start() {
        this.syncState();
        this.syncReports();
        setInterval(() => this.syncState(), this.pollInterval);
        setInterval(() => this.syncReports(), this.reportPollInterval);
    }
}

// Bootstrap Application
document.addEventListener('DOMContentLoaded', () => {
    const app = new DashboardController();
    app.start();
});
