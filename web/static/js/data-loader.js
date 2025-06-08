// Call records functions
function loadCalls() {
    const tbody = document.getElementById('calls-tbody');
    tbody.innerHTML = '<tr><td colspan="6" class="loading"><img src="/static/Meiko.png" alt="Meiko" style="width: 24px; height: 24px; opacity: 0.7; vertical-align: middle; margin-right: 8px;">Meiko is scanning call records...</td></tr>';

    fetch('/api/calls?limit=50')
        .then(response => response.json())
        .then(data => {
            displayCalls(data.calls);
        })
        .catch(error => {
            tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; color: var(--text-muted);"><img src="/static/MeikoConfused.png" alt="Confused Meiko" style="width: 32px; height: 32px; opacity: 0.3; vertical-align: middle; margin-right: 8px;">Meiko couldn\'t load call records</td></tr>';
        });
}

function displayCalls(calls) {
    const tbody = document.getElementById('calls-tbody');
    
    if (!calls || calls.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; color: var(--text-muted);"><img src="/static/MeikoConfused.png" alt="Confused Meiko" style="width: 32px; height: 32px; opacity: 0.5; vertical-align: middle; margin-right: 8px;">Meiko hasn\'t detected any calls yet</td></tr>';
        return;
    }

    tbody.innerHTML = calls.map(call => {
        const timestamp = new Date(call.timestamp);
        const duration = call.duration + 's';
        const transcription = call.transcription ? 
            (call.transcription.length > 50 ? call.transcription.substring(0, 50) + '...' : call.transcription) :
            'No transcription';

        // Format timestamp consistently with 12-hour format
        const formattedTime = timestamp.toLocaleString('en-US', {
            weekday: 'short',
            month: 'short', 
            day: 'numeric',
            hour: '2-digit', 
            minute: '2-digit',
            hour12: true
        });

        return `
            <tr onclick="showCallDetails(${call.id})" style="cursor: pointer;">
                <td>${formattedTime}</td>
                <td>${call.talkgroup_group || 'Unknown'}</td>
                <td>${call.talkgroup_alias || call.talkgroup_id || 'Unknown'}</td>
                <td>${duration}</td>
                <td>${call.frequency || 'N/A'}</td>
                <td>${transcription}</td>
            </tr>
        `;
    }).join('');
}

// Analytics functions
function loadAnalytics() {
    updateStatCards();
    loadDepartmentStats();
}

function updateStatCards() {
    fetch('/api/stats')
        .then(response => response.json())
        .then(stats => {
            document.getElementById('total-calls-stat').textContent = stats.total_calls || '0';
            document.getElementById('calls-today-stat').textContent = stats.calls_today || '0';
            document.getElementById('active-talkgroups-stat').textContent = Object.keys(stats.talkgroups || {}).length;
            
            // Format uptime (convert from nanoseconds to seconds)
            const uptimeNanos = stats.uptime || 0;
            const uptimeSeconds = Math.floor(uptimeNanos / 1000000000);
            const hours = Math.floor(uptimeSeconds / 3600);
            const minutes = Math.floor((uptimeSeconds % 3600) / 60);
            document.getElementById('system-uptime-stat').textContent = `${hours}h ${minutes}m`;
        })
        .catch(error => {
            console.error('Failed to load stats:', error);
        });
}

// Summary functionality moved to timeline - keeping for backwards compatibility
function refreshSummary() {
    // Redirect to timeline summaries
    console.log('Summary refresh redirected to timeline');
    if (typeof refreshTimelineSummaries === 'function') {
        refreshTimelineSummaries();
    }
}

function loadDepartmentStats() {
    const container = document.getElementById('department-stats');
    
    fetch('/api/stats')
        .then(response => response.json())
        .then(stats => {
            if (stats.talkgroups && Object.keys(stats.talkgroups).length > 0) {
                const talkgroups = Object.entries(stats.talkgroups)
                    .sort((a, b) => b[1] - a[1])
                    .slice(0, 10);

                container.innerHTML = talkgroups.map(([name, count]) => `
                    <div style="display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid var(--border-secondary);">
                        <span>${name}</span>
                        <span style="font-family: var(--font-mono); color: var(--accent-blue);">${count}</span>
                    </div>
                `).join('');
            } else {
                container.innerHTML = '<div class="empty-state"><img src="/static/MeikoConfused.png" alt="Confused Meiko" style="width: 48px; height: 48px; opacity: 0.5; margin-bottom: 12px;"><p>Meiko found no department data</p></div>';
            }
        })
        .catch(error => {
            container.innerHTML = '<div class="empty-state"><img src="/static/MeikoConfused.png" alt="Confused Meiko" style="width: 48px; height: 48px; opacity: 0.3; margin-bottom: 12px;"><p>Meiko couldn\'t load department stats</p></div>';
        });
}

// Console functions
function loadConsole() {
    loadSystemStats();
    loadLogs();
}

function loadSystemStats() {
    fetch('/api/stats')
        .then(response => response.json())
        .then(stats => {
            document.getElementById('cpu-usage').textContent = (stats.cpu || 0).toFixed(1) + '%';
            document.getElementById('memory-usage').textContent = (stats.memory || 0).toFixed(1) + '%';
            document.getElementById('disk-usage').textContent = (stats.disk || 0).toFixed(1) + '%';
            document.getElementById('temperature').textContent = (stats.temperature || 0).toFixed(1) + '°C';
        })
        .catch(error => {
            console.error('Failed to load system stats:', error);
        });
}

function updateSystemStats() {
    if (currentTab === 'console' || currentTab === 'analytics') {
        loadSystemStats();
    }
}

function loadLogs() {
    const container = document.getElementById('logs-container');
    container.innerHTML = '<div class="loading"><img src="/static/Meiko.png" alt="Meiko" style="width: 32px; height: 32px; opacity: 0.7; margin-right: 12px;">Meiko is fetching system logs...</div>';

    fetch('/api/logs')
        .then(response => response.json())
        .then(data => {
            if (data.logs && data.logs.length > 0) {
                container.innerHTML = data.logs.map(log => {
                    const timestamp = new Date(log.timestamp).toLocaleTimeString('en-US', {
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                        hour12: true
                    });
                    const levelColor = getLevelColor(log.level);
                    // Clean message of any existing level indicators to prevent duplication
                    const cleanMessage = cleanLogMessage(log.message, log.level);
                    return `
                        <div style="margin-bottom: 8px; font-family: var(--font-mono); font-size: 12px;">
                            <span style="color: var(--text-muted);">[${timestamp}]</span>
                            <span style="color: ${levelColor}; font-weight: 500;">[${log.level}]</span>
                            <span style="color: var(--text-secondary);">[${log.component}]</span>
                            <span style="color: var(--text-primary);">${cleanMessage}</span>
                        </div>
                    `;
                }).join('');
                container.scrollTop = container.scrollHeight;
            } else {
                container.innerHTML = '<div class="empty-state"><img src="/static/MeikoConfused.png" alt="Confused Meiko" style="width: 48px; height: 48px; opacity: 0.5; margin-bottom: 12px;"><p>Meiko found no logs to display</p></div>';
            }
        })
        .catch(error => {
            container.innerHTML = '<div class="empty-state"><img src="/static/MeikoConfused.png" alt="Confused Meiko" style="width: 48px; height: 48px; opacity: 0.3; margin-bottom: 12px;"><p>Meiko couldn\'t access system logs</p></div>';
        });
}

// Helper function to get color for log level
function getLevelColor(level) {
    switch (level.toUpperCase()) {
        case 'ERROR': return '#ff6b6b';
        case 'WARN': case 'WARNING': return '#ffa726';
        case 'INFO': return '#42a5f5';
        case 'DEBUG': return '#66bb6a';
        case 'SUCCESS': return '#4caf50';
        default: return 'var(--text-secondary)';
    }
}

// Helper function to clean log messages of duplicate level indicators
function cleanLogMessage(message, logLevel) {
    if (!message) return message;
    
    let cleaned = message;
    
    // Remove level indicators that match the current log level at the start of message
    const levelPattern = new RegExp(`^\\s*\\[${logLevel}\\]\\s*`, 'i');
    cleaned = cleaned.replace(levelPattern, '');
    
    // Remove any duplicate level brackets like [[INFO]] at start
    const doubleLevelPattern = new RegExp(`^\\s*\\[\\[${logLevel}\\]\\]\\s*`, 'i');
    cleaned = cleaned.replace(doubleLevelPattern, '');
    
    // Remove generic level patterns at start of message (any level)
    const genericLevelPattern = /^\s*\[(INFO|WARN|ERROR|DEBUG|SUCCESS)\]\s*/i;
    cleaned = cleaned.replace(genericLevelPattern, '');
    
    // Remove double brackets patterns like [[LEVEL]]
    const doubleGenericPattern = /^\s*\[\[(INFO|WARN|ERROR|DEBUG|SUCCESS)\]\]\s*/i;
    cleaned = cleaned.replace(doubleGenericPattern, '');
    
    // Remove multiple consecutive level indicators
    const multipleLevelPattern = /(\[(INFO|WARN|ERROR|DEBUG|SUCCESS)\]\s*){2,}/gi;
    cleaned = cleaned.replace(multipleLevelPattern, '');
    
    return cleaned.trim();
} 