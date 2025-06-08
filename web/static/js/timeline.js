// Timeline functions
let isLoadingTimeline = false;
let timelineSummaries = {};
let showSummaries = true;

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

    // Use correct API endpoint format and increase limit
    const timelineUrl = `/api/timeline/${currentDate}?limit=200`;
    
    // Load both timeline events and summaries
    Promise.all([
        fetch(timelineUrl).then(r => {
            if (!r.ok) {
                throw new Error(`HTTP ${r.status}: ${r.statusText}`);
            }
            return r.json();
        }),
        loadTimelineSummaries(currentDate)
    ])
    .then(([timelineData, summariesData]) => {
        timelineSummaries = summariesData.summaries || {};
        displayEnhancedTimeline(timelineData.events);
        console.log(`Timeline loaded: ${timelineData.events?.length || 0} events, ${Object.keys(timelineSummaries).length} summaries for ${currentDate}`);
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

function loadTimelineSummaries(date) {
    return fetch(`/api/timeline/summaries/${date}`)
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .catch(error => {
            console.warn('Failed to load timeline summaries:', error);
            return { summaries: {} };
        });
}

function displayEnhancedTimeline(events) {
    const container = document.getElementById('timeline-container');
    
    if (!events || events.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <img src="/static/MeikoConfused.png" alt="Confused Meiko" style="width: 64px; height: 64px; opacity: 0.5; margin-bottom: 16px;">
                <p>Meiko is waiting for activity...</p>
                <small style="color: var(--text-muted);">No events found for ${currentDate}</small>
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
        
        const srcMatch = currentTimelineAudio.src.match(/\/api\/calls\/(\d+)\/audio/);
        if (srcMatch) {
            currentCallId = parseInt(srcMatch[1]);
        }
    }
    
    // Organize events by hour and create enhanced timeline
    const organizedTimeline = organizeTimelineByHour(events);
    container.innerHTML = createEnhancedTimelineHTML(organizedTimeline);
    
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

function organizeTimelineByHour(events) {
    const hourlyGroups = {};
    
    // Group events by hour
    events.forEach(event => {
        const eventTime = new Date(event.timestamp);
        const hour = eventTime.getHours();
        
        if (!hourlyGroups[hour]) {
            hourlyGroups[hour] = {
                hour: hour,
                events: [],
                summary: timelineSummaries[hour] || null
            };
        }
        
        hourlyGroups[hour].events.push(event);
    });
    
    // Sort by hour (most recent first - higher hour numbers first)
    return Object.values(hourlyGroups).sort((a, b) => b.hour - a.hour);
}

function createEnhancedTimelineHTML(organizedTimeline) {
    if (!organizedTimeline.length) {
        return '<div class="empty-state"><p>No events to display</p></div>';
    }
    
    return organizedTimeline.map(hourGroup => {
        return createHourlyTimelineBlock(hourGroup);
    }).join('');
}

function createHourlyTimelineBlock(hourGroup) {
    const hour = hourGroup.hour;
    const events = hourGroup.events;
    const summary = hourGroup.summary;
    
    const hourLabel = formatHourLabel(hour);
    const categoryTags = summary?.categories ? createCategoryTags(summary.categories) : '';
    
    let html = `<div class="timeline-hour-block" data-hour="${hour}">`;
    
    // Hour header with summary
    html += `<div class="timeline-hour-header">`;
    html += `<div class="timeline-hour-info">`;
    html += `<div class="timeline-hour-label">${hourLabel}</div>`;
    html += `<div class="timeline-hour-meta">${events.length} event${events.length !== 1 ? 's' : ''}${categoryTags}</div>`;
    html += `</div>`;
    
    if (summary && showSummaries) {
        html += `<div class="timeline-hour-actions">`;
        html += `<button class="btn-small summary-toggle" onclick="toggleHourSummary(${hour})" title="Toggle AI Summary">`;
        html += `<i class="fas fa-robot"></i>`;
        html += `</button>`;
        html += `</div>`;
    }
    
    html += `</div>`;
    
    // AI Summary section (initially hidden on mobile)
    if (summary && showSummaries) {
        html += `<div class="timeline-hour-summary" id="summary-${hour}">`;
        html += `<div class="summary-content">`;
        html += `<div class="summary-header">`;
        html += `<i class="fas fa-robot"></i>`;
        html += `<span class="summary-title">AI Analysis</span>`;
        html += `<span class="summary-meta">${summary.call_count} calls analyzed</span>`;
        html += `</div>`;
        html += `<div class="summary-text">${summary.summary}</div>`;
        html += `</div>`;
        html += `</div>`;
    }
    
    // Events for this hour (sort by time within hour, newest first)
    const sortedEvents = events.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
    html += `<div class="timeline-hour-events">`;
    html += sortedEvents.map(event => createTimelineItem(event)).join('');
    html += `</div>`;
    
    html += `</div>`;
    
    return html;
}

function formatHourLabel(hour) {
    const hour12 = hour === 0 ? 12 : hour > 12 ? hour - 12 : hour;
    const ampm = hour < 12 ? 'AM' : 'PM';
    return `${hour12}:00 ${ampm}`;
}

function createCategoryTags(categories) {
    if (!categories || categories.length === 0) return '';
    
    const categoryColors = {
        'POLICE': 'var(--police-color)',
        'FIRE': 'var(--fire-color)',
        'EMS': 'var(--ems-color)',
        'MEDICAL': 'var(--ems-color)',
        'EMERGENCY': 'var(--emergency-color)',
        'TRAFFIC': 'var(--accent-yellow)',
        'PUBLIC_WORKS': 'var(--public-works-color)',
        'OTHER': 'var(--text-muted)'
    };
    
    const tags = categories.slice(0, 3).map(category => {
        const color = categoryColors[category] || categoryColors['OTHER'];
        return `<span class="category-tag" style="color: ${color}; border-color: ${color}">${category}</span>`;
    }).join('');
    
    return ` ${tags}`;
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

function toggleHourSummary(hour) {
    const summaryElement = document.getElementById(`summary-${hour}`);
    const toggleButton = document.querySelector(`[onclick="toggleHourSummary(${hour})"]`);
    
    if (summaryElement) {
        const isVisible = summaryElement.classList.contains('visible');
        
        if (isVisible) {
            summaryElement.classList.remove('visible');
            toggleButton.innerHTML = '<i class="fas fa-robot"></i>';
            toggleButton.setAttribute('title', 'Show AI Summary');
        } else {
            summaryElement.classList.add('visible');
            toggleButton.innerHTML = '<i class="fas fa-robot" style="color: var(--accent-blue);"></i>';
            toggleButton.setAttribute('title', 'Hide AI Summary');
        }
    }
}

function toggleAllSummaries() {
    showSummaries = !showSummaries;
    loadTimeline(); // Reload to apply the change
}

function refreshTimelineSummaries() {
    const container = document.getElementById('timeline-container');
    
    loadTimelineSummaries(currentDate)
        .then(data => {
            timelineSummaries = data.summaries || {};
            console.log('Timeline summaries refreshed:', Object.keys(timelineSummaries).length);
            
            // Update existing summary sections
            Object.keys(timelineSummaries).forEach(hour => {
                const summaryElement = document.getElementById(`summary-${hour}`);
                if (summaryElement) {
                    const summary = timelineSummaries[hour];
                    summaryElement.innerHTML = `
                        <div class="summary-content">
                            <div class="summary-header">
                                <i class="fas fa-robot"></i>
                                <span class="summary-title">AI Analysis</span>
                                <span class="summary-meta">${summary.call_count} calls analyzed</span>
                            </div>
                            <div class="summary-text">${summary.summary}</div>
                        </div>
                    `;
                }
            });
        })
        .catch(error => {
            console.error('Failed to refresh summaries:', error);
        });
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

// Add event listener for date picker changes
function initTimelineDatePicker() {
    const dateInput = document.getElementById('timeline-date');
    if (dateInput) {
        // Set initial date
        dateInput.value = currentDate;
        
        // Add change event listener
        dateInput.addEventListener('change', function() {
            currentDate = this.value;
            console.log('Date changed to:', currentDate);
            loadTimeline();
        });
    }
} 