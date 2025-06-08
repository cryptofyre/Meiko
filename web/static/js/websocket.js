// WebSocket connection
let wsReconnectAttempts = 0;
const maxReconnectAttempts = 5;

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    ws = new WebSocket(wsUrl);
    
    ws.onopen = function() {
        console.log('WebSocket connected');
        updateSystemStatus('online');
        wsReconnectAttempts = 0; // Reset reconnect attempts on successful connection
        
        // Update Meiko status
        updateMeikoStatus("System connected", "Real-time monitoring active");
    };
    
    ws.onmessage = function(event) {
        const data = JSON.parse(event.data);
        handleWebSocketMessage(data);
    };
    
    ws.onclose = function(event) {
        console.log('WebSocket disconnected', event.code, event.reason);
        updateSystemStatus('offline');
        
        // Update Meiko status
        updateMeikoStatus("Connection lost", "Attempting to reconnect...");
        
        // Attempt to reconnect with exponential backoff
        if (wsReconnectAttempts < maxReconnectAttempts) {
            wsReconnectAttempts++;
            const delay = Math.min(1000 * Math.pow(2, wsReconnectAttempts), 30000); // Max 30 second delay
            console.log(`Attempting to reconnect in ${delay}ms (attempt ${wsReconnectAttempts}/${maxReconnectAttempts})`);
            setTimeout(connectWebSocket, delay);
        } else {
            console.log('Max reconnection attempts reached');
            updateMeikoStatus("Connection failed", "Manual refresh required");
        }
    };
    
    ws.onerror = function(error) {
        console.error('WebSocket error:', error);
        updateSystemStatus('offline');
        
        // Update Meiko status  
        updateMeikoStatus("Connection error", "Network issues detected");
    };
}

// Periodic connectivity check
function startConnectivityMonitor() {
    setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN) {
            // Connection is healthy
            updateSystemStatus('online');
        } else if (ws && ws.readyState === WebSocket.CONNECTING) {
            // Currently connecting
            updateSystemStatus('connecting');
        } else {
            // Connection is closed or failed
            updateSystemStatus('offline');
        }
    }, 5000); // Check every 5 seconds
}

// WebSocket message handling
function handleWebSocketMessage(data) {
    switch(data.type) {
        case 'stats_update':
            if (currentTab === 'console' || currentTab === 'analytics') {
                updateSystemStats();
            }
            break;
        case 'new_call':
            // Meiko reacts to new calls
            updateMeikoStatus("New transmission detected!", "Processing call data");
            setTimeout(() => {
                document.getElementById('meiko-status-text').textContent = "Ready for monitoring";
                document.getElementById('meiko-status-subtitle').textContent = "Emergency services active";
            }, 5000);

            // Always refresh timeline if it's the current tab
            if (currentTab === 'timeline') {
                // Add a small delay to ensure the backend has processed the call
                setTimeout(() => {
                    loadTimeline(true); // Silent refresh to avoid loading indicators
                }, 500);
            }
            
            // Always refresh calls if it's the current tab
            if (currentTab === 'calls') {
                setTimeout(() => {
                    loadCalls();
                }, 500);
            }
            
            // Update analytics if it's the current tab
            if (currentTab === 'analytics') {
                setTimeout(() => {
                    updateStatCards();
                }, 1000);
            }
            
            // Handle live scanner for new calls
            handleWebSocketMessageForLiveScanner(data);
            
            console.log('New call received:', data.data);
            break;
        case 'live_scanner_event':
            // Handle live scanner specific events
            handleWebSocketMessageForLiveScanner(data);
            break;
        default:
            console.log('Unknown message type:', data.type);
    }
}

function testWebSocketBroadcast() {
    console.log('Testing WebSocket broadcast...');
    updateMeikoStatus("Testing WebSocket", "Triggering manual broadcast");
    
    fetch('/api/debug/broadcast-latest', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        }
    })
    .then(response => response.json())
    .then(data => {
        console.log('WebSocket test broadcast result:', data);
        updateMeikoStatus("WebSocket test sent", "Check console for results");
    })
    .catch(error => {
        console.error('WebSocket test failed:', error);
        updateMeikoStatus("WebSocket test failed", "Check console for details");
    });
} 