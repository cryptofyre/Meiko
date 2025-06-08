// Timeline functions
let isLoadingTimeline = false;

function loadTimeline(silent = false) {
    // Prevent multiple simultaneous loads
    if (isLoadingTimeline) {
        return;
    }
    
    isLoadingTimeline = true;
    const container = document.getElementById('timeline-container');
    
    if (!silent) {
        container.innerHTML = '<div class="loading"><img src="/static/Meiko.png" alt="Meiko" style="width: 32px; height: 32px; opacity: 0.7; margin-right: 12px;">Meiko is scanning for events...</div>';
    }

    fetch(`/api/timeline?date=${currentDate}`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .then(data => {
            displayTimeline(data.events);
            console.log(`Timeline loaded: ${data.events?.length || 0} events`);
        })
        .catch(error => {
            console.error('Timeline load error:', error);
            container.innerHTML = `
                <div class="empty-state">
                    <img src="/static/MeikoConfused.png" alt="Confused Meiko" style="width: 64px; height: 64px; opacity: 0.3; margin-bottom: 16px;">
                    <p>Meiko encountered an error!</p>
                    <small style="color: var(--text-muted);">Failed to load timeline events: ${error.message}</small>
                </div>
            `;
        })
        .finally(() => {
            isLoadingTimeline = false;
        });
}

function displayTimeline(events) {
    const container = document.getElementById('timeline-container');
    
    if (!events || events.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <img src="/static/MeikoConfused.png" alt="Confused Meiko" style="width: 64px; height: 64px; opacity: 0.5; margin-bottom: 16px;">
                <p>Meiko is waiting for activity...</p>
                <small style="color: var(--text-muted);">No events found for this date</small>
            </div>
        `;
        return;
    }

    // Check if this is an auto-refresh with new data
    const currentItemCount = container.querySelectorAll('.timeline-item').length;
    const newItemCount = events.length;
    
    // Store current audio state before refresh
    let wasPlaying = false;
    let currentCallId = null;
    let audioCurrentTime = 0;
    
    if (currentTimelineAudio && currentTimelineButton) {
        wasPlaying = !currentTimelineAudio.paused;
        audioCurrentTime = currentTimelineAudio.currentTime;
        
        // Extract call ID from the audio src
        const srcMatch = currentTimelineAudio.src.match(/\/api\/calls\/(\d+)\/audio/);
        if (srcMatch) {
            currentCallId = parseInt(srcMatch[1]);
        }
    }
    
    container.innerHTML = events.map(event => createTimelineItem(event)).join('');
    
    // Restore audio state if it was playing
    if (wasPlaying && currentCallId) {
        setTimeout(() => {
            const newButton = container.querySelector(`[onclick="playCallAudio(${currentCallId})"]`);
            if (newButton && currentTimelineAudio) {
                currentTimelineButton = newButton;
                currentTimelineAudio.currentTime = audioCurrentTime;
                if (currentTimelineAudio.paused) {
                    currentTimelineAudio.play();
                }
                newButton.innerHTML = '<i class="fas fa-pause"></i> PAUSE';
            }
        }, 100);
    }
    
    // Flash the timeline briefly to indicate new data if items were added
    if (newItemCount > currentItemCount && currentItemCount > 0) {
        container.style.transition = 'background-color 0.3s ease';
        container.style.backgroundColor = 'rgba(66, 165, 245, 0.1)';
        setTimeout(() => {
            container.style.backgroundColor = '';
            setTimeout(() => {
                container.style.transition = '';
            }, 300);
        }, 300);
    }
}

function createTimelineItem(event) {
    const timestamp = new Date(event.timestamp);
    const timeString = timestamp.toLocaleTimeString('en-US', { 
        hour: '2-digit', 
        minute: '2-digit',
        hour12: true
    });

    // Build tags
    let tagsHTML = '';
    if (event.data) {
        if (event.data.talkgroup) {
            tagsHTML += `<span class="timeline-tag">${event.data.talkgroup}</span>`;
        }
        if (event.data.frequency) {
            tagsHTML += `<span class="timeline-tag">${event.data.frequency}</span>`;
        }
        if (event.data.duration) {
            tagsHTML += `<span class="timeline-tag">${event.data.duration}s</span>`;
        }
    }

    // Build controls for call events
    let controlsHTML = '';
    if (event.type === 'call' && event.data && event.data.call_id) {
        controlsHTML = `
            <div class="timeline-controls">
                <button class="btn-small play-btn" onclick="playCallAudio(${event.data.call_id})">
                    <i class="fas fa-play"></i> PLAY
                </button>
                <button class="btn-small" onclick="showCallDetails(${event.data.call_id})">
                    <i class="fas fa-info-circle"></i> DETAILS
                </button>
            </div>
        `;
    }

    const serviceType = event.data && event.data.service_type ? event.data.service_type : 'OTHER';

    return `
        <div class="timeline-item" data-service="${serviceType}">
            <div class="timeline-time">${timeString}</div>
            <div class="timeline-icon">
                <i class="fas fa-${event.icon}"></i>
            </div>
            <div class="timeline-content">
                <div class="timeline-title">${event.title}</div>
                <div class="timeline-description">${event.description}</div>
                <div class="timeline-tags">${tagsHTML}</div>
                ${controlsHTML}
            </div>
        </div>
    `;
}

// Date controls
function setTimelineDate(period) {
    const today = new Date();
    switch(period) {
        case 'today':
            currentDate = today.toISOString().split('T')[0];
            break;
        case 'yesterday':
            const yesterday = new Date(today);
            yesterday.setDate(yesterday.getDate() - 1);
            currentDate = yesterday.toISOString().split('T')[0];
            break;
    }
    document.getElementById('timeline-date').value = currentDate;
    loadTimeline();
}

function refreshTimeline() {
    const dateInput = document.getElementById('timeline-date');
    if (dateInput.value) {
        currentDate = dateInput.value;
    }
    loadTimeline();
} 