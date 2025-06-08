// Live Scanner state
let liveScanner = {
    isActive: false,
    currentAudio: null,
    waveformCanvas: null,
    waveformContext: null,
    animationId: null,
    transcriptionItems: [],
    lastCallId: null,
    volume: 0.75
};

// Initialize Live Scanner
function initLiveScanner() {
    liveScanner.waveformCanvas = document.getElementById('waveform-canvas');
    liveScanner.waveformContext = liveScanner.waveformCanvas.getContext('2d');
    
    // Set up canvas sizing
    resizeWaveformCanvas();
    window.addEventListener('resize', resizeWaveformCanvas);
    
    // Set up volume control
    const volumeSlider = document.getElementById('master-volume');
    const volumeDisplay = document.getElementById('volume-display');
    
    volumeSlider.addEventListener('input', (e) => {
        liveScanner.volume = e.target.value / 100;
        volumeDisplay.textContent = e.target.value + '%';
        
        if (liveScanner.currentAudio) {
            liveScanner.currentAudio.volume = liveScanner.volume;
        }
    });
    
    // Initialize waveform
    drawStandbyWaveform();
}

function resizeWaveformCanvas() {
    const container = document.getElementById('waveform-container');
    const canvas = liveScanner.waveformCanvas;
    
    canvas.width = container.clientWidth;
    canvas.height = container.clientHeight;
    
    if (!liveScanner.isActive) {
        drawStandbyWaveform();
    }
}

function drawStandbyWaveform() {
    const ctx = liveScanner.waveformContext;
    const canvas = liveScanner.waveformCanvas;
    
    ctx.fillStyle = 'var(--bg-tertiary)';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    
    // Draw static waveform
    ctx.strokeStyle = '#333333';
    ctx.lineWidth = 1;
    ctx.beginPath();
    
    const centerY = canvas.height / 2;
    const segments = 200;
    
    for (let i = 0; i < segments; i++) {
        const x = (i / segments) * canvas.width;
        const amplitude = Math.sin(i * 0.1) * 10 + Math.sin(i * 0.05) * 5;
        const y = centerY + amplitude;
        
        if (i === 0) {
            ctx.moveTo(x, y);
        } else {
            ctx.lineTo(x, y);
        }
    }
    
    ctx.stroke();
}

function drawLiveWaveform(audioData) {
    const ctx = liveScanner.waveformContext;
    const canvas = liveScanner.waveformCanvas;
    
    // Clear canvas
    ctx.fillStyle = '#111111';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    
    if (!audioData) return;
    
    // Draw waveform
    ctx.strokeStyle = '#00d4ff';
    ctx.lineWidth = 2;
    ctx.beginPath();
    
    const centerY = canvas.height / 2;
    const barWidth = canvas.width / audioData.length;
    
    for (let i = 0; i < audioData.length; i++) {
        const amplitude = audioData[i] * centerY;
        const x = i * barWidth;
        
        ctx.moveTo(x, centerY - amplitude);
        ctx.lineTo(x, centerY + amplitude);
    }
    
    ctx.stroke();
    
    // Add time indicator
    updateTimeIndicator();
}

function drawRealWaveform(waveformData) {
    const ctx = liveScanner.waveformContext;
    const canvas = liveScanner.waveformCanvas;
    
    // Clear canvas with gradient background
    const gradient = ctx.createLinearGradient(0, 0, 0, canvas.height);
    gradient.addColorStop(0, '#0a0a0a');
    gradient.addColorStop(1, '#111111');
    ctx.fillStyle = gradient;
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    
    if (!waveformData || waveformData.length === 0) return;
    
    // Draw waveform bars
    const barWidth = canvas.width / waveformData.length;
    const centerY = canvas.height / 2;
    
    // Create gradient for waveform
    const waveGradient = ctx.createLinearGradient(0, 0, 0, canvas.height);
    waveGradient.addColorStop(0, '#00d4ff');
    waveGradient.addColorStop(0.5, '#00ff88');
    waveGradient.addColorStop(1, '#00d4ff');
    
    ctx.fillStyle = waveGradient;
    
    for (let i = 0; i < waveformData.length; i++) {
        const amplitude = Math.abs(waveformData[i]) * centerY * 0.8;
        const x = i * barWidth;
        
        // Draw vertical bar
        ctx.fillRect(x, centerY - amplitude, barWidth - 1, amplitude * 2);
    }
    
    // Add reflection effect
    ctx.globalAlpha = 0.3;
    ctx.scale(1, -1);
    ctx.translate(0, -canvas.height);
    ctx.fillStyle = waveGradient;
    
    for (let i = 0; i < waveformData.length; i++) {
        const amplitude = Math.abs(waveformData[i]) * centerY * 0.4;
        const x = i * barWidth;
        
        ctx.fillRect(x, centerY - amplitude, barWidth - 1, amplitude);
    }
    
    // Reset transformations
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.globalAlpha = 1;
    
    // Add time indicator
    updateTimeIndicator();
}

function updateTimeIndicator() {
    const now = new Date();
    const timeString = now.toLocaleTimeString('en-US', { 
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
    });
    document.getElementById('current-time').textContent = timeString;
}

function toggleLiveScanner() {
    if (liveScanner.isActive) {
        stopLiveScanner();
    } else {
        startLiveScanner();
    }
}

function startLiveScanner() {
    liveScanner.isActive = true;
    
    // Update UI
    const button = document.getElementById('scanner-toggle');
    const status = document.getElementById('scanner-status');
    
    button.innerHTML = '<i class="fas fa-stop"></i> STOP SCANNING';
    button.classList.add('btn-primary');
    
    status.className = 'scanner-indicator live';
    status.querySelector('span').textContent = 'LIVE';
    
    // Start waveform animation
    animateWaveform();
    
    // Start scanning simulation
    startLiveScannerMonitoring();
    
    // Update Meiko status
    updateMeikoStatus("Live scanner activated", "Monitoring all frequencies");
    
    console.log('Live scanner started - isActive:', liveScanner.isActive);
    
    // Show status in UI
    document.getElementById('active-frequency').textContent = 'Live Scanner Active - Waiting for calls';
}

function stopLiveScanner() {
    liveScanner.isActive = false;
    
    // Stop any playing audio
    if (liveScanner.currentAudio) {
        liveScanner.currentAudio.pause();
        liveScanner.currentAudio = null;
    }
    
    // Stop animation
    if (liveScanner.animationId) {
        cancelAnimationFrame(liveScanner.animationId);
        liveScanner.animationId = null;
    }
    
    // Update UI
    const button = document.getElementById('scanner-toggle');
    const status = document.getElementById('scanner-status');
    const waveformContainer = document.getElementById('waveform-container');
    const currentCallInfo = document.getElementById('current-call-info');
    
    button.innerHTML = '<i class="fas fa-play"></i> START SCANNING';
    button.classList.remove('btn-primary');
    
    status.className = 'scanner-indicator standby';
    status.querySelector('span').textContent = 'STANDBY';
    
    waveformContainer.classList.remove('playing');
    currentCallInfo.classList.remove('active');
    
    // Reset to standby waveform
    drawStandbyWaveform();
    
    // Update Meiko status
    updateMeikoStatus("Scanner stopped", "Standing by");
    
    console.log('Live scanner stopped');
}

function animateWaveform() {
    if (!liveScanner.isActive) return;
    
    // Generate fake waveform data for demonstration
    const dataPoints = 100;
    const audioData = new Array(dataPoints);
    
    for (let i = 0; i < dataPoints; i++) {
        // Create more realistic looking audio data
        const baseWave = Math.sin(Date.now() * 0.001 + i * 0.1) * 0.3;
        const noise = (Math.random() - 0.5) * 0.1;
        audioData[i] = baseWave + noise;
    }
    
    drawLiveWaveform(audioData);
    
    liveScanner.animationId = requestAnimationFrame(animateWaveform);
}

function startLiveScannerMonitoring() {
    if (!liveScanner.isActive) return;
    
    // Simulate scanning activity
    setInterval(() => {
        if (!liveScanner.isActive) return;
        
        // Randomly update frequency display to simulate scanning
        const frequencies = ['453.100', '453.200', '453.300', '460.100', '460.200'];
        const services = ['Police Dispatch', 'Fire Department', 'EMS Services', 'Public Works', 'Airport Operations'];
        
        if (Math.random() > 0.8) { // 20% chance to update
            const randomFreq = frequencies[Math.floor(Math.random() * frequencies.length)];
            const randomService = services[Math.floor(Math.random() * services.length)];
            
            document.getElementById('active-frequency').textContent = 
                `Scanning ${randomFreq} • ${randomService}`;
                
            // Briefly flash the frequency display
            const freqElement = document.getElementById('active-frequency');
            freqElement.style.color = 'var(--accent-blue)';
            setTimeout(() => {
                freqElement.style.color = '';
            }, 500);
        }
        
        // Update time indicator
        updateTimeIndicator();
        
    }, 2000); // Update every 2 seconds
}

function playLiveCall(callData) {
    console.log('playLiveCall called with:', callData);
    
    if (!liveScanner.isActive) {
        console.log('Live scanner not active, aborting playback');
        return;
    }
    
    // Stop previous audio
    if (liveScanner.currentAudio) {
        console.log('Stopping previous audio');
        liveScanner.currentAudio.pause();
    }
    
    // Create new audio instance
    const audioUrl = `/api/calls/${callData.id}/audio`;
    console.log('Creating audio instance for URL:', audioUrl);
    liveScanner.currentAudio = new Audio(audioUrl);
    liveScanner.currentAudio.volume = liveScanner.volume;
    
    // Update current call info
    showCurrentCallInfo(callData);
    
    // Add visual feedback
    const waveformContainer = document.getElementById('waveform-container');
    waveformContainer.classList.add('playing');
    
    // Set up audio event listeners
    liveScanner.currentAudio.addEventListener('loadstart', () => {
        console.log('Audio loading started');
    });
    
    liveScanner.currentAudio.addEventListener('canplay', () => {
        console.log('Audio can play');
    });
    
    liveScanner.currentAudio.addEventListener('loadeddata', () => {
        console.log('Audio data loaded');
    });
    
    liveScanner.currentAudio.addEventListener('ended', () => {
        console.log('Audio playback ended');
        waveformContainer.classList.remove('playing');
        hideCurrentCallInfo();
    });
    
    liveScanner.currentAudio.addEventListener('error', (e) => {
        console.error('Live audio playback error:', e);
        console.error('Audio error details:', liveScanner.currentAudio.error);
        waveformContainer.classList.remove('playing');
        hideCurrentCallInfo();
        
        // Show user-friendly error message
        updateMeikoStatus("Audio playback failed", "Check console for details");
    });
    
    liveScanner.currentAudio.addEventListener('play', () => {
        console.log('Audio started playing');
    });
    
    liveScanner.currentAudio.addEventListener('pause', () => {
        console.log('Audio paused');
    });
    
    // Attempt to play the audio
    console.log('Attempting to play audio...');
    const playPromise = liveScanner.currentAudio.play();
    
    if (playPromise !== undefined) {
        playPromise
            .then(() => {
                console.log('Audio playback started successfully');
            })
            .catch(error => {
                console.error('Failed to play live audio:', error);
                
                // Handle autoplay restriction
                if (error.name === 'NotAllowedError') {
                    console.log('Autoplay blocked by browser - user interaction required');
                    updateMeikoStatus("Click to enable audio", "Browser autoplay blocked");
                    
                    // Show a play button overlay or notification
                    showAutoplayNotification(callData);
                } else {
                    console.error('Other audio error:', error);
                    updateMeikoStatus("Audio error", error.message);
                }
                
                waveformContainer.classList.remove('playing');
                hideCurrentCallInfo();
            });
    }
    
    liveScanner.lastCallId = callData.id;
    console.log('Updated lastCallId to:', liveScanner.lastCallId);
}

function showAutoplayNotification(callData) {
    // Create a temporary notification to enable audio
    const notification = document.createElement('div');
    notification.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        background: var(--accent-blue);
        color: white;
        padding: 16px;
        border-radius: 4px;
        z-index: 1001;
        cursor: pointer;
        animation: slideIn 0.3s ease-out;
    `;
    notification.innerHTML = `
        <div style="font-weight: 600; margin-bottom: 4px;">🔊 Enable Audio</div>
        <div style="font-size: 12px;">Click to allow audio playback</div>
    `;
    
    notification.addEventListener('click', () => {
        // User interaction - now we can play audio
        liveScanner.currentAudio.play()
            .then(() => {
                console.log('Audio enabled by user interaction');
                updateMeikoStatus("Audio enabled", "Live scanning active");
            })
            .catch(err => {
                console.error('Still failed to play after user interaction:', err);
            });
        
        document.body.removeChild(notification);
    });
    
    document.body.appendChild(notification);
    
    // Auto-remove after 10 seconds
    setTimeout(() => {
        if (notification.parentNode) {
            document.body.removeChild(notification);
        }
    }, 10000);
}

function showCurrentCallInfo(callData) {
    const currentCallInfo = document.getElementById('current-call-info');
    const title = document.getElementById('call-info-title');
    const meta = document.getElementById('call-info-meta');
    const duration = document.getElementById('call-info-duration');
    
    title.textContent = callData.talkgroup_alias || 'Unknown';
    meta.textContent = `${callData.frequency} • ${new Date(callData.timestamp).toLocaleTimeString()}`;
    duration.textContent = `${callData.duration}s`;
    
    currentCallInfo.classList.add('active');
}

function hideCurrentCallInfo() {
    const currentCallInfo = document.getElementById('current-call-info');
    currentCallInfo.classList.remove('active');
}

function addTranscriptionToFeed(callData) {
    if (!callData.transcription) return;
    
    const feed = document.getElementById('transcription-feed');
    
    // Remove empty state if present
    const emptyState = feed.querySelector('.empty-transcription');
    if (emptyState) {
        emptyState.remove();
    }
    
    // Create transcription item
    const item = document.createElement('div');
    item.className = 'transcription-item new';
    
    const timestamp = new Date(callData.timestamp).toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    });
    
    item.innerHTML = `
        <div class="transcription-meta">
            <span class="transcription-time">${timestamp}</span>
            <span class="transcription-talkgroup">${callData.talkgroup_alias || 'Unknown'}</span>
        </div>
        <div class="transcription-text">${callData.transcription}</div>
    `;
    
    // Add to top of feed
    feed.insertBefore(item, feed.firstChild);
    
    // Remove 'new' class after animation
    setTimeout(() => {
        item.classList.remove('new');
    }, 2000);
    
    // Keep only last 50 items
    liveScanner.transcriptionItems.unshift(item);
    if (liveScanner.transcriptionItems.length > 50) {
        const oldItem = liveScanner.transcriptionItems.pop();
        if (oldItem.parentNode) {
            oldItem.parentNode.removeChild(oldItem);
        }
    }
    
    // Auto-scroll to top
    feed.scrollTop = 0;
}

function clearTranscriptionFeed() {
    const feed = document.getElementById('transcription-feed');
    feed.innerHTML = `
        <div class="empty-transcription">
            <i class="fas fa-microphone-slash"></i>
            <p>Waiting for communications...</p>
            <small>Live transcriptions will appear here as calls come in</small>
        </div>
    `;
    liveScanner.transcriptionItems = [];
}

function testAudioPlayback() {
    console.log('Testing audio playback...');
    updateMeikoStatus("Testing audio", "Fetching recent call for test");
    
    // Get the most recent call for testing
    fetch('/api/calls?limit=1')
        .then(response => response.json())
        .then(data => {
            if (data.calls && data.calls.length > 0) {
                const testCall = data.calls[0];
                console.log('Using call for test:', testCall);
                
                updateMeikoStatus("Playing test audio", `Testing with call #${testCall.id}`);
                
                // Create test audio
                const testAudio = new Audio(`/api/calls/${testCall.id}/audio`);
                testAudio.volume = liveScanner.volume;
                
                testAudio.addEventListener('loadeddata', () => {
                    console.log('Test audio loaded successfully');
                });
                
                testAudio.addEventListener('error', (e) => {
                    console.error('Test audio error:', e);
                    updateMeikoStatus("Audio test failed", "Check console for details");
                });
                
                testAudio.addEventListener('ended', () => {
                    console.log('Test audio finished');
                    updateMeikoStatus("Audio test complete", "Audio system working");
                });
                
                // Attempt to play
                testAudio.play()
                    .then(() => {
                        console.log('Test audio playing successfully');
                        updateMeikoStatus("Audio test playing", "Audio system working");
                        
                        // Update current call info for test
                        showCurrentCallInfo(testCall);
                        
                        // Show waveform
                        const waveformContainer = document.getElementById('waveform-container');
                        waveformContainer.classList.add('playing');
                        
                        // Remove visual feedback when done
                        testAudio.addEventListener('ended', () => {
                            waveformContainer.classList.remove('playing');
                            hideCurrentCallInfo();
                        });
                        
                    })
                    .catch(error => {
                        console.error('Test audio failed to play:', error);
                        if (error.name === 'NotAllowedError') {
                            updateMeikoStatus("Audio blocked", "Browser requires user interaction");
                            showAutoplayNotification(testCall);
                        } else {
                            updateMeikoStatus("Audio error", error.message);
                        }
                    });
            } else {
                updateMeikoStatus("No calls available", "No audio files to test with");
                console.log('No calls available for testing');
            }
        })
        .catch(error => {
            console.error('Failed to fetch calls for test:', error);
            updateMeikoStatus("Test failed", "Could not fetch call data");
        });
}

// Enhanced WebSocket message handling for live scanner
function handleWebSocketMessageForLiveScanner(data) {
    console.log('Live Scanner received WebSocket message:', data);
    
    if (data.type === 'new_call' && liveScanner.isActive) {
        const callData = data.data;
        const liveScannerData = data.live_scanner;
        
        console.log('Live Scanner processing new call:', callData);
        console.log('Live Scanner data:', liveScannerData);
        
        // Add to transcription feed
        addTranscriptionToFeed(callData);
        
        // Update frequency display
        if (liveScannerData && liveScannerData.frequency_info) {
            const freqInfo = liveScannerData.frequency_info;
            document.getElementById('active-frequency').textContent = 
                `${freqInfo.frequency} • ${freqInfo.description}`;
            console.log('Updated frequency display:', freqInfo);
        }
        
        // Use real waveform data if available
        if (liveScannerData && liveScannerData.waveform_data) {
            console.log('Drawing real waveform data:', liveScannerData.waveform_data.length, 'points');
            drawRealWaveform(liveScannerData.waveform_data);
        }
        
        // Auto-play if this is a new call and auto-play is enabled
        if (callData.id !== liveScanner.lastCallId) {
            console.log('Attempting to play new call:', callData.id, 'Previous call:', liveScanner.lastCallId);
            
            if (liveScannerData && liveScannerData.should_auto_play) {
                console.log('Auto-play enabled, starting playback in 500ms');
                // Small delay to ensure audio file is ready
                setTimeout(() => {
                    playLiveCall(callData);
                }, 500);
            } else {
                console.log('Auto-play disabled or no live scanner data');
            }
        } else {
            console.log('Same call ID, skipping auto-play');
        }
    } else if (data.type === 'new_call' && !liveScanner.isActive) {
        console.log('Live Scanner not active, ignoring new call');
    }
    
    // Handle live scanner specific events
    if (data.type === 'live_scanner_event') {
        console.log('Received live scanner event:', data.event, data.data);
        switch(data.event) {
            case 'frequency_change':
                updateFrequencyDisplay(data.data);
                break;
            case 'signal_strength':
                updateSignalStrength(data.data);
                break;
            case 'scanner_status':
                updateScannerStatus(data.data);
                break;
        }
    }
}

function updateFrequencyDisplay(freqData) {
    document.getElementById('active-frequency').textContent = 
        `${freqData.frequency} • ${freqData.description || 'Monitoring'}`;
}

function updateSignalStrength(signalData) {
    // Could be used to update a signal strength indicator
    console.log('Signal strength:', signalData);
}

function updateScannerStatus(statusData) {
    // Update scanner status based on backend events
    const status = document.getElementById('scanner-status');
    if (statusData.active) {
        status.className = 'scanner-indicator live';
        status.querySelector('span').textContent = 'LIVE';
    } else {
        status.className = 'scanner-indicator standby';
        status.querySelector('span').textContent = 'STANDBY';
    }
} 