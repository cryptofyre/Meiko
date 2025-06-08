// Call details modal
function showCallDetails(callId) {
    fetch(`/api/calls/${callId}`)
        .then(response => response.json())
        .then(call => {
            displayCallDetails(call);
            document.getElementById('call-modal').style.display = 'block';
        })
        .catch(error => {
            console.error('Failed to load call details:', error);
            const container = document.getElementById('call-details-content');
            container.innerHTML = `
                <div class="empty-state">
                    <img src="/static/MeikoConfused.png" alt="Confused Meiko" style="width: 64px; height: 64px; opacity: 0.3; margin-bottom: 16px;">
                    <p>Meiko couldn't load call details</p>
                    <small style="color: var(--text-muted);">Failed to fetch call information</small>
                </div>
            `;
            document.getElementById('call-modal').style.display = 'block';
        });
}

function displayCallDetails(call) {
    const container = document.getElementById('call-details-content');
    const timestamp = new Date(call.timestamp);
    const formattedTime = timestamp.toLocaleString('en-US', {
        weekday: 'long',
        year: 'numeric', 
        month: 'long',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: true
    });
    
    const duration = formatDuration(call.duration);
    
    container.innerHTML = `
        <div class="call-meta-grid">
            <div class="call-meta-item">
                <div class="call-meta-label">Call ID</div>
                <div class="call-meta-value">#${call.id}</div>
            </div>
            <div class="call-meta-item">
                <div class="call-meta-label">Duration</div>
                <div class="call-meta-value">${duration}</div>
            </div>
            <div class="call-meta-item">
                <div class="call-meta-label">Frequency</div>
                <div class="call-meta-value">${call.frequency || 'Unknown'}</div>
            </div>
            <div class="call-meta-item">
                <div class="call-meta-label">System</div>
                <div class="call-meta-value">${call.talkgroup_group || 'Unknown'}</div>
            </div>
        </div>

        <div class="call-details">
            <dt>Timestamp</dt>
            <dd>${formattedTime}</dd>
            <dt>Talkgroup</dt>
            <dd>${call.talkgroup_alias || call.talkgroup_id}</dd>
            <dt>Filename</dt>
            <dd>${call.filename}</dd>
        </div>

        <div class="custom-audio-player">
            <div class="audio-controls">
                <button class="play-button" id="play-btn-${call.id}">
                    <i class="fas fa-play"></i>
                </button>
                <div class="audio-progress" id="progress-${call.id}">
                    <div class="audio-progress-fill" id="progress-fill-${call.id}"></div>
                </div>
                <div class="audio-time" id="time-${call.id}">0:00 / 0:00</div>
                <div class="audio-volume">
                    <i class="fas fa-volume-up"></i>
                    <input type="range" class="volume-slider" id="volume-${call.id}" min="0" max="100" value="100">
                </div>
            </div>
            <audio id="audio-${call.id}" preload="metadata">
                <source src="/api/calls/${call.id}/audio" type="audio/mpeg">
            </audio>
        </div>

        ${call.transcription ? `
            <div class="call-transcription-section">
                <div class="transcription-header">
                    <i class="fas fa-quote-left"></i>
                    Transcription
                </div>
                <div class="transcription-content">
                    ${call.transcription}
                </div>
            </div>
        ` : ''}
    `;

    // Initialize custom audio player
    initCustomAudioPlayer(call.id);
} 