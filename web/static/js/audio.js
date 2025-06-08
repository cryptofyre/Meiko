// Global audio player for timeline
let currentTimelineAudio = null;
let currentTimelineButton = null;

function playCallAudio(callId) {
    const button = event.target.closest('.play-btn');
    
    // Stop any currently playing audio first
    if (currentTimelineAudio && !currentTimelineAudio.paused) {
        currentTimelineAudio.pause();
        currentTimelineAudio.currentTime = 0;
        
        // Reset the old button if it exists and is different from current
        if (currentTimelineButton && currentTimelineButton !== button) {
            currentTimelineButton.innerHTML = '<i class="fas fa-play"></i> PLAY';
        }
    }
    
    // If clicking the same audio that was already loaded
    if (currentTimelineAudio && currentTimelineAudio.src.includes(`/api/calls/${callId}/audio`) && currentTimelineButton === button) {
        if (currentTimelineAudio.paused) {
            // Resume playback
            currentTimelineAudio.play().then(() => {
                button.innerHTML = '<i class="fas fa-pause"></i> PAUSE';
            }).catch(error => {
                console.error('Failed to resume audio:', error);
                button.innerHTML = '<i class="fas fa-play"></i> PLAY';
            });
        } else {
            // Pause playback
            currentTimelineAudio.pause();
            button.innerHTML = '<i class="fas fa-play"></i> PLAY';
        }
        return;
    }
    
    // Create new audio instance for different call
    if (currentTimelineAudio) {
        currentTimelineAudio.pause();
        currentTimelineAudio = null;
    }
    
    currentTimelineAudio = new Audio(`/api/calls/${callId}/audio`);
    currentTimelineButton = button;
    
    // Set up event listeners
    currentTimelineAudio.addEventListener('ended', () => {
        if (currentTimelineButton) {
            currentTimelineButton.innerHTML = '<i class="fas fa-play"></i> PLAY';
        }
    });
    
    currentTimelineAudio.addEventListener('pause', () => {
        if (currentTimelineButton) {
            currentTimelineButton.innerHTML = '<i class="fas fa-play"></i> PLAY';
        }
    });
    
    currentTimelineAudio.addEventListener('play', () => {
        if (currentTimelineButton) {
            currentTimelineButton.innerHTML = '<i class="fas fa-pause"></i> PAUSE';
        }
    });
    
    currentTimelineAudio.addEventListener('error', () => {
        console.error('Audio failed to load');
        if (currentTimelineButton) {
            currentTimelineButton.innerHTML = '<i class="fas fa-play"></i> PLAY';
        }
    });
    
    // Start playback
    currentTimelineAudio.play().then(() => {
        button.innerHTML = '<i class="fas fa-pause"></i> PAUSE';
    }).catch(error => {
        console.error('Failed to play audio:', error);
        button.innerHTML = '<i class="fas fa-play"></i> PLAY';
    });
}

// Custom audio player functionality
function initCustomAudioPlayer(callId) {
    const audio = document.getElementById(`audio-${callId}`);
    const playBtn = document.getElementById(`play-btn-${callId}`);
    const progress = document.getElementById(`progress-${callId}`);
    const progressFill = document.getElementById(`progress-fill-${callId}`);
    const timeDisplay = document.getElementById(`time-${callId}`);
    const volumeSlider = document.getElementById(`volume-${callId}`);

    let isPlaying = false;

    // Play/pause functionality
    playBtn.addEventListener('click', () => {
        if (isPlaying) {
            audio.pause();
        } else {
            audio.play();
        }
    });

    // Update play button state
    audio.addEventListener('play', () => {
        isPlaying = true;
        playBtn.innerHTML = '<i class="fas fa-pause"></i>';
    });

    audio.addEventListener('pause', () => {
        isPlaying = false;
        playBtn.innerHTML = '<i class="fas fa-play"></i>';
    });

    // Update progress and time
    audio.addEventListener('timeupdate', () => {
        if (audio.duration) {
            const progress = (audio.currentTime / audio.duration) * 100;
            progressFill.style.width = progress + '%';
            
            const currentTime = formatTime(audio.currentTime);
            const totalTime = formatTime(audio.duration);
            timeDisplay.textContent = `${currentTime} / ${totalTime}`;
        }
    });

    // Progress bar click
    progress.addEventListener('click', (e) => {
        if (audio.duration) {
            const rect = progress.getBoundingClientRect();
            const x = e.clientX - rect.left;
            const clickProgress = x / rect.width;
            audio.currentTime = clickProgress * audio.duration;
        }
    });

    // Volume control
    volumeSlider.addEventListener('input', (e) => {
        audio.volume = e.target.value / 100;
    });

    // Load metadata
    audio.addEventListener('loadedmetadata', () => {
        const totalTime = formatTime(audio.duration);
        timeDisplay.textContent = `0:00 / ${totalTime}`;
    });
}

// Helper function to format time
function formatTime(seconds) {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

// Helper function to format duration
function formatDuration(seconds) {
    if (seconds < 60) {
        return `${seconds}s`;
    }
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}m ${secs}s`;
} 