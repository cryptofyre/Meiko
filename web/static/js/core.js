// Global state
let ws = null;
let currentTab = 'timeline';
let currentDate = new Date().toISOString().split('T')[0];

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    connectWebSocket();
    startConnectivityMonitor(); // Start monitoring WebSocket connection
    loadSystemStats();
    loadTimeline();
    startMeikoPersonality();
    setInterval(updateSystemStats, 5000); // Update every 5 seconds
});

// Meiko personality system
function startMeikoPersonality() {
    const statusTexts = [
        "Ready for monitoring",
        "Scanning frequencies",
        "Listening for activity", 
        "Standing by",
        "Monitoring channels",
        "Awaiting transmissions"
    ];

    const subtitleTexts = [
        "Emergency services active",
        "All systems operational", 
        "Scanner network online",
        "Communications clear",
        "Ready to assist",
        "Monitoring in progress"
    ];

    let statusIndex = 0;
    let subtitleIndex = 0;

    setInterval(() => {
        if (Math.random() > 0.7) { // 30% chance to update
            statusIndex = (statusIndex + 1) % statusTexts.length;
            document.getElementById('meiko-status-text').textContent = statusTexts[statusIndex];
        }
        
        if (Math.random() > 0.8) { // 20% chance to update
            subtitleIndex = (subtitleIndex + 1) % subtitleTexts.length;
            document.getElementById('meiko-status-subtitle').textContent = subtitleTexts[subtitleIndex];
        }
    }, 10000); // Check every 10 seconds
}

// Tab switching
function switchTab(tabName) {
    // Update navigation
    document.querySelectorAll('.nav-tab').forEach(tab => {
        tab.classList.remove('active');
    });
    event.target.classList.add('active');

    // Update content
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });
    document.getElementById(tabName).classList.add('active');

    currentTab = tabName;

    // Load content for the selected tab
    switch(tabName) {
        case 'timeline':
            loadTimeline();
            break;
        case 'live-scanner':
            if (!liveScanner.waveformCanvas) {
                initLiveScanner();
            }
            // Auto-start scanner when tab is opened for testing
            if (!liveScanner.isActive) {
                console.log('Live Scanner tab opened - ready to start');
                updateMeikoStatus("Live Scanner ready", "Click START SCANNING to begin");
            }
            break;
        case 'calls':
            loadCalls();
            break;
        case 'analytics':
            loadAnalytics();
            break;
        case 'console':
            loadConsole();
            break;
    }
}

// System status updates
function updateSystemStatus(status) {
    const statusEl = document.getElementById('system-status');
    const circle = statusEl.querySelector('i');
    
    if (status === 'online') {
        statusEl.className = 'status-indicator online';
        circle.className = 'fas fa-circle';
    } else if (status === 'connecting') {
        statusEl.className = 'status-indicator connecting';
        circle.className = 'fas fa-spinner fa-spin';
    } else {
        statusEl.className = 'status-indicator offline';
        circle.className = 'fas fa-times';
    }
}

// Update Meiko's status messages
function updateMeikoStatus(statusText, subtitleText) {
    document.getElementById('meiko-status-text').textContent = statusText;
    document.getElementById('meiko-status-subtitle').textContent = subtitleText;
}

// Utility functions
function refreshCalls() {
    loadCalls();
}

function refreshSummary() {
    loadSummary();
}

function refreshLogs() {
    loadLogs();
}

// Close modal when clicking outside
window.onclick = function(event) {
    const modal = document.getElementById('call-modal');
    if (event.target === modal) {
        closeCallModal();
    }
}

function closeCallModal() {
    document.getElementById('call-modal').style.display = 'none';
} 