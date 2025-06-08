// Live Scanner state
let liveScanner = {
    isActive: false,
    currentAudio: null,
    waveformCanvas: null,
    waveformContext: null,
    animationId: null,
    transcriptionItems: [],
    lastCallId: null,
    volume: 0.75,
    callQueue: [],
    isPlaying: false,
    currentCall: null,
    // Enhanced transcription state
    currentTranscriptionElement: null,
    transcriptionHighlightInterval: null,
    currentWordIndex: 0,
    transcriptionWords: []
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
    
    // Set up keyboard shortcuts
    setupKeyboardShortcuts();
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
    
    // Clear queue and reset state
    clearCallQueue();
    liveScanner.isPlaying = false;
    liveScanner.currentCall = null;
    
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
    
    // If already playing, add to queue
    if (liveScanner.isPlaying && liveScanner.currentAudio && !liveScanner.currentAudio.paused) {
        console.log('Audio currently playing, adding call to queue:', callData.id);
        addCallToQueue(callData);
        return;
    }
    
    // Start playing immediately
    startPlayingCall(callData);
}

function addCallToQueue(callData) {
    // Avoid duplicate calls in queue
    const existingIndex = liveScanner.callQueue.findIndex(call => call.id === callData.id);
    if (existingIndex === -1) {
        liveScanner.callQueue.push(callData);
        console.log('Added call to queue. Queue length:', liveScanner.callQueue.length);
        updateQueueDisplay();
    } else {
        console.log('Call already in queue, skipping:', callData.id);
    }
}

function startPlayingCall(callData) {
    console.log('Starting playback for call:', callData.id);
    
    liveScanner.isPlaying = true;
    liveScanner.currentCall = callData;
    
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
        
        // Mark as not playing and clear current call
        liveScanner.isPlaying = false;
        liveScanner.currentCall = null;
        
        // Play next call in queue if available
        playNextInQueue();
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
    
    // Add enhanced event handlers for transcription features
    enhanceAudioEventHandlers(liveScanner.currentAudio, callData);
    
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
                
                // Mark as not playing and clear current call
                liveScanner.isPlaying = false;
                liveScanner.currentCall = null;
            });
    }
    
    liveScanner.lastCallId = callData.id;
    console.log('Updated lastCallId to:', liveScanner.lastCallId);
}

function playNextInQueue() {
    if (liveScanner.callQueue.length > 0) {
        const nextCall = liveScanner.callQueue.shift();
        console.log('Playing next call from queue:', nextCall.id, 'Remaining in queue:', liveScanner.callQueue.length);
        updateQueueDisplay();
        
        // Small delay to ensure clean transitions
        setTimeout(() => {
            startPlayingCall(nextCall);
        }, 100);
    } else {
        console.log('Queue is empty, no more calls to play');
        updateQueueDisplay();
    }
}

function updateQueueDisplay() {
    const queueContainer = document.getElementById('call-queue-container');
    const queueCount = document.getElementById('queue-count');
    const queueList = document.getElementById('queue-list');
    
    if (!queueContainer) return; // Elements might not exist yet
    
    if (liveScanner.callQueue.length > 0) {
        queueContainer.classList.add('active');
        queueCount.textContent = liveScanner.callQueue.length;
        
        // Update queue list
        queueList.innerHTML = '';
        liveScanner.callQueue.slice(0, 3).forEach((call, index) => {
            const item = document.createElement('div');
            item.className = 'queue-item';
            
            const timestamp = new Date(call.timestamp).toLocaleTimeString('en-US', {
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit',
                hour12: false
            });
            
            item.innerHTML = `
                <div class="queue-item-meta">
                    <span class="queue-position">#${index + 1}</span>
                    <span class="queue-time">${timestamp}</span>
                </div>
                <div class="queue-item-title">${call.talkgroup_alias || 'Unknown'}</div>
                <div class="queue-item-duration">${call.duration}s</div>
            `;
            
            queueList.appendChild(item);
        });
        
        // Add "and more" indicator if queue is longer
        if (liveScanner.callQueue.length > 3) {
            const moreItem = document.createElement('div');
            moreItem.className = 'queue-more';
            moreItem.textContent = `+${liveScanner.callQueue.length - 3} more`;
            queueList.appendChild(moreItem);
        }
    } else {
        queueContainer.classList.remove('active');
    }
}

function clearCallQueue() {
    liveScanner.callQueue = [];
    updateQueueDisplay();
    console.log('Call queue cleared');
}

function skipCurrentCall() {
    if (liveScanner.isPlaying && liveScanner.currentAudio) {
        console.log('Skipping current call');
        liveScanner.currentAudio.pause();
        liveScanner.currentAudio.currentTime = liveScanner.currentAudio.duration || 0;
        
        // Trigger ended event to move to next call
        liveScanner.currentAudio.dispatchEvent(new Event('ended'));
    }
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
    
    // Add live transcription display
    showLiveTranscription(callData);
    
    // Highlight the corresponding transcription in the feed
    highlightActiveTranscriptionInFeed(callData.id);
}

function hideCurrentCallInfo() {
    const currentCallInfo = document.getElementById('current-call-info');
    currentCallInfo.classList.remove('active');
    
    // Hide live transcription and stop highlighting
    hideLiveTranscription();
    
    // Remove active highlighting from feed
    clearActiveTranscriptionHighlight();
}

function showLiveTranscription(callData) {
    if (!callData.transcription) return;
    
    // Create or update the live transcription container
    let liveTranscriptionContainer = document.getElementById('live-transcription-container');
    if (!liveTranscriptionContainer) {
        liveTranscriptionContainer = document.createElement('div');
        liveTranscriptionContainer.id = 'live-transcription-container';
        liveTranscriptionContainer.className = 'live-transcription-container';
        
        // Insert after current call info
        const currentCallInfo = document.getElementById('current-call-info');
        currentCallInfo.parentNode.insertBefore(liveTranscriptionContainer, currentCallInfo.nextSibling);
    }
    
    liveTranscriptionContainer.innerHTML = `
        <div class="live-transcription-header">
            <div class="live-transcription-title">
                <i class="fas fa-waveform-lines"></i>
                Live Transcription
            </div>
            <div class="transcription-controls">
                <div class="transcription-progress">
                    <div class="progress-bar">
                        <div class="progress-fill" id="transcription-progress-fill"></div>
                    </div>
                    <span class="progress-text" id="transcription-progress-text">0:00 / ${callData.duration}:00</span>
                </div>
                <button class="btn-small" onclick="skipCurrentCall()" title="Skip to next call">
                    <i class="fas fa-forward"></i>
                </button>
            </div>
        </div>
        <div class="live-transcription-text" id="live-transcription-text">
            ${formatTranscriptionForHighlighting(callData.transcription)}
        </div>
    `;
    
    liveTranscriptionContainer.classList.add('active');
    
    // Start word-by-word highlighting simulation
    startTranscriptionHighlighting(callData);
}

function hideLiveTranscription() {
    const container = document.getElementById('live-transcription-container');
    if (container) {
        container.classList.remove('active');
    }
    
    // Stop highlighting animation
    stopTranscriptionHighlighting();
}

function formatTranscriptionForHighlighting(transcription) {
    // Split transcription into words and wrap each in a span for highlighting
    const words = transcription.split(/(\s+|[.,!?;:])/);
    liveScanner.transcriptionWords = words.filter(word => word.trim().length > 0);
    
    return words.map((word, index) => {
        if (word.trim().length === 0) {
            return word; // Preserve whitespace
        }
        return `<span class="transcription-word" data-word-index="${index}">${word}</span>`;
    }).join('');
}

function startTranscriptionHighlighting(callData) {
    if (!liveScanner.currentAudio) return;
    
    liveScanner.currentWordIndex = 0;
    const totalDuration = callData.duration * 1000; // Convert to milliseconds
    const totalWords = liveScanner.transcriptionWords.length;
    const wordsPerSecond = totalWords / callData.duration;
    
    // Clear any existing interval
    stopTranscriptionHighlighting();
    
    // Start highlighting animation
    liveScanner.transcriptionHighlightInterval = setInterval(() => {
        if (!liveScanner.currentAudio || liveScanner.currentAudio.paused) {
            return;
        }
        
        const currentTime = liveScanner.currentAudio.currentTime;
        const progressPercent = (currentTime / callData.duration) * 100;
        
        // Update progress bar
        updateTranscriptionProgress(currentTime, callData.duration, progressPercent);
        
        // Calculate which words should be highlighted based on audio progress
        const expectedWordIndex = Math.floor(currentTime * wordsPerSecond);
        
        // Highlight words up to the current position
        highlightWordsUpTo(expectedWordIndex);
        
        // Auto-scroll if needed
        scrollToCurrentWord();
        
    }, 100); // Update every 100ms for smooth animation
}

function stopTranscriptionHighlighting() {
    if (liveScanner.transcriptionHighlightInterval) {
        clearInterval(liveScanner.transcriptionHighlightInterval);
        liveScanner.transcriptionHighlightInterval = null;
    }
    
    // Clear all highlighting
    const words = document.querySelectorAll('.transcription-word');
    words.forEach(word => {
        word.classList.remove('highlighted', 'current');
    });
    
    liveScanner.currentWordIndex = 0;
}

function highlightWordsUpTo(wordIndex) {
    const words = document.querySelectorAll('.transcription-word');
    
    words.forEach((word, index) => {
        if (index < wordIndex) {
            word.classList.add('highlighted');
            word.classList.remove('current');
        } else if (index === wordIndex) {
            word.classList.add('current');
            word.classList.remove('highlighted');
        } else {
            word.classList.remove('highlighted', 'current');
        }
    });
}

function updateTranscriptionProgress(currentTime, totalDuration, progressPercent) {
    const progressFill = document.getElementById('transcription-progress-fill');
    const progressText = document.getElementById('transcription-progress-text');
    
    if (progressFill) {
        progressFill.style.width = `${Math.min(progressPercent, 100)}%`;
    }
    
    if (progressText) {
        const currentMin = Math.floor(currentTime / 60);
        const currentSec = Math.floor(currentTime % 60);
        const totalMin = Math.floor(totalDuration / 60);
        const totalSec = Math.floor(totalDuration % 60);
        
        progressText.textContent = `${currentMin}:${currentSec.toString().padStart(2, '0')} / ${totalMin}:${totalSec.toString().padStart(2, '0')}`;
    }
}

function scrollToCurrentWord() {
    const currentWord = document.querySelector('.transcription-word.current');
    const container = document.getElementById('live-transcription-text');
    
    if (currentWord && container) {
        const containerRect = container.getBoundingClientRect();
        const wordRect = currentWord.getBoundingClientRect();
        
        // Check if the current word is out of view
        if (wordRect.bottom > containerRect.bottom || wordRect.top < containerRect.top) {
            currentWord.scrollIntoView({
                behavior: 'smooth',
                block: 'center'
            });
        }
    }
}

function highlightActiveTranscriptionInFeed(callId) {
    // Remove any existing active highlighting
    clearActiveTranscriptionHighlight();
    
    // Find and highlight the transcription item for the current call
    const transcriptionItems = document.querySelectorAll('.transcription-item');
    transcriptionItems.forEach(item => {
        // We'll need to store the call ID in the transcription item
        if (item.dataset.callId === callId.toString()) {
            item.classList.add('active-playback');
            
            // Scroll the item into view in the feed
            item.scrollIntoView({
                behavior: 'smooth',
                block: 'center'
            });
        }
    });
}

function clearActiveTranscriptionHighlight() {
    const activeItems = document.querySelectorAll('.transcription-item.active-playback');
    activeItems.forEach(item => {
        item.classList.remove('active-playback');
    });
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
    item.dataset.callId = callData.id; // Store call ID for highlighting
    
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
            <span class="transcription-duration">${callData.duration}s</span>
        </div>
        <div class="transcription-text">${callData.transcription}</div>
        <div class="transcription-actions">
            <button class="transcription-action-btn" onclick="playCallFromFeed('${callData.id}')" title="Play this call">
                <i class="fas fa-play"></i>
            </button>
            <button class="transcription-action-btn" onclick="showCallDetails('${callData.id}')" title="View details">
                <i class="fas fa-info-circle"></i>
            </button>
        </div>
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
    
    // Auto-scroll to top only if not currently playing something
    if (!liveScanner.isPlaying) {
        feed.scrollTop = 0;
    }
}

// New function to play a call directly from the transcription feed
function playCallFromFeed(callId) {
    console.log('Playing call from transcription feed:', callId);
    
    // Create a mock call data object (in real implementation, you'd fetch this)
    const callData = {
        id: callId,
        talkgroup_alias: 'Manual Playback',
        frequency: '000.000',
        timestamp: new Date().toISOString(),
        duration: 10 // Default duration, real implementation would fetch this
    };
    
    // Stop current playback if any
    if (liveScanner.currentAudio) {
        liveScanner.currentAudio.pause();
    }
    
    // Start playing the selected call
    startPlayingCall(callData);
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
    
    // Also clear any live transcription display
    hideLiveTranscription();
    clearActiveTranscriptionHighlight();
}

// Utility function to show call details (placeholder)
function showCallDetails(callId) {
    console.log('Showing details for call:', callId);
    // In a real implementation, this would open a modal or navigate to a details page
    updateMeikoStatus("Call details", `Viewing details for call #${callId}`);
}

// Enhanced audio event handlers
function enhanceAudioEventHandlers(audio, callData) {
    // Add time update handler for transcription highlighting
    audio.addEventListener('timeupdate', () => {
        if (liveScanner.transcriptionHighlightInterval) {
            // The highlighting is already handled by the interval
            // This is just for any additional time-based features
        }
    });
    
    // Handle seeking (if user could scrub through audio)
    audio.addEventListener('seeked', () => {
        if (callData && liveScanner.transcriptionHighlightInterval) {
            // Restart highlighting from the new position
            stopTranscriptionHighlighting();
            startTranscriptionHighlighting(callData);
        }
    });
    
    // Handle pause/play for highlighting
    audio.addEventListener('pause', () => {
        console.log('Audio paused, pausing transcription highlighting');
    });
    
    audio.addEventListener('play', () => {
        console.log('Audio resumed, resuming transcription highlighting');
    });
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
        
        // Add to transcription feed immediately (regardless of playback)
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

// Add keyboard shortcuts for live scanner
function setupKeyboardShortcuts() {
    document.addEventListener('keydown', (e) => {
        // Only handle shortcuts when live scanner tab is active
        if (currentTab !== 'live-scanner') return;
        
        // Prevent default for our handled keys
        const handledKeys = ['Space', 'ArrowRight', 'ArrowLeft', 'Escape', 'KeyS'];
        if (handledKeys.includes(e.code)) {
            e.preventDefault();
        }
        
        switch(e.code) {
            case 'Space': // Spacebar - toggle scanner or pause/resume current audio
                if (liveScanner.isActive && liveScanner.currentAudio) {
                    // If audio is playing, pause/resume it
                    if (liveScanner.currentAudio.paused) {
                        liveScanner.currentAudio.play();
                    } else {
                        liveScanner.currentAudio.pause();
                    }
                } else {
                    // Toggle scanner
                    toggleLiveScanner();
                }
                break;
                
            case 'ArrowRight': // Right arrow - skip current call
                if (liveScanner.isPlaying) {
                    skipCurrentCall();
                }
                break;
                
            case 'ArrowLeft': // Left arrow - restart current call
                if (liveScanner.currentAudio) {
                    liveScanner.currentAudio.currentTime = 0;
                }
                break;
                
            case 'Escape': // Escape - stop scanner
                if (liveScanner.isActive) {
                    stopLiveScanner();
                }
                break;
                
            case 'KeyS': // S - toggle scanner
                toggleLiveScanner();
                break;
                
            case 'KeyC': // C - clear transcription feed
                if (e.shiftKey) {
                    clearTranscriptionFeed();
                }
                break;
                
            case 'KeyT': // T - test audio
                if (e.shiftKey) {
                    testAudioPlayback();
                }
                break;
        }
    });
}

// Show keyboard shortcuts help
function showKeyboardShortcuts() {
    // Check if modal already exists
    let modal = document.getElementById('shortcuts-modal');
    if (modal) {
        modal.style.display = 'block';
        return;
    }
    
    const shortcuts = [
        { key: 'SPACEBAR', action: 'Play/Pause Audio or Toggle Scanner' },
        { key: '→', action: 'Skip Current Call' },
        { key: '←', action: 'Restart Current Call' },
        { key: 'ESC', action: 'Stop Scanner' },
        { key: 'S', action: 'Toggle Scanner' },
        { key: 'SHIFT + C', action: 'Clear Transcription Feed' },
        { key: 'SHIFT + T', action: 'Test Audio' }
    ];
    
    const shortcutsHtml = shortcuts.map(s => 
        `<div class="shortcut-item">
            <kbd class="key">${s.key}</kbd>
            <span class="action">${s.action}</span>
        </div>`
    ).join('');
    
    // Create modal
    modal = document.createElement('div');
    modal.id = 'shortcuts-modal';
    modal.className = 'modal';
    modal.style.cssText = `
        display: block;
        position: fixed;
        z-index: 1000;
        left: 0;
        top: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.5);
        animation: fadeIn 0.15s ease-out;
    `;
    
    modal.innerHTML = `
        <div class="modal-content" style="
            background: var(--bg-primary);
            margin: 10% auto;
            padding: 0;
            border: 1px solid var(--border-primary);
            border-radius: 4px;
            width: 90%;
            max-width: 500px;
            box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
            animation: slideIn 0.2s ease-out;
        ">
            <div class="modal-header" style="
                padding: 16px 20px;
                border-bottom: 1px solid var(--border-secondary);
                display: flex;
                align-items: center;
                justify-content: space-between;
            ">
                <h3 style="
                    margin: 0;
                    color: var(--text-primary);
                    font-size: 16px;
                    font-weight: 600;
                    display: flex;
                    align-items: center;
                    gap: 8px;
                ">
                    <i class="fas fa-keyboard" style="color: var(--accent-blue);"></i>
                    Keyboard Shortcuts
                </h3>
                <button class="modal-close" onclick="closeShortcutsModal()" style="
                    background: none;
                    border: none;
                    color: var(--text-secondary);
                    font-size: 18px;
                    cursor: pointer;
                    padding: 4px;
                    border-radius: 2px;
                    line-height: 1;
                ">
                    <i class="fas fa-times"></i>
                </button>
            </div>
            <div class="modal-body" style="
                padding: 20px;
                max-height: 400px;
                overflow-y: auto;
            ">
                <div class="shortcuts-list" style="
                    display: flex;
                    flex-direction: column;
                    gap: 4px;
                ">
                    ${shortcutsHtml}
                </div>
                <div style="
                    margin-top: 16px;
                    padding-top: 16px;
                    border-top: 1px solid var(--border-secondary);
                    font-size: 12px;
                    color: var(--text-muted);
                    text-align: center;
                ">
                    These shortcuts only work when the Live Scanner tab is active
                </div>
            </div>
        </div>
    `;
    
    // Add event listeners
    modal.addEventListener('click', (e) => {
        if (e.target === modal) {
            closeShortcutsModal();
        }
    });
    
    // Add ESC key listener
    const escListener = (e) => {
        if (e.key === 'Escape') {
            closeShortcutsModal();
            document.removeEventListener('keydown', escListener);
        }
    };
    document.addEventListener('keydown', escListener);
    
    // Add to page
    document.body.appendChild(modal);
    
    updateMeikoStatus("Keyboard shortcuts", "Showing available hotkeys");
}

// Close shortcuts modal
function closeShortcutsModal() {
    const modal = document.getElementById('shortcuts-modal');
    if (modal) {
        modal.style.animation = 'fadeOut 0.15s ease-out';
        setTimeout(() => {
            if (modal.parentNode) {
                document.body.removeChild(modal);
            }
        }, 150);
        
        updateMeikoStatus("Ready for monitoring", "Live scanner controls available");
    }
} 