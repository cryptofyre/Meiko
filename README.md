# Meiko - Unified SDRTrunk & Transcription System

🎤 **Meiko** is a unified Go application that integrates SDRTrunk and audio transcription services. It provides a local-first, robust, and user-friendly experience for monitoring and transcribing radio communications.

## Features

### Core Functionality
- **SDRTrunk Process Management**: Launches and monitors SDRTrunk as a child process
- **File System Monitoring**: Watches for new audio recordings from SDRTrunk
- **Dual Transcription Modes**: 
  - Local transcription using faster-whisper
  - Remote transcription via API endpoints
- **Discord Integration**: Real-time notifications and system status updates
- **Database Storage**: SQLite database for call records and transcriptions
- **System Monitoring**: Health checks and performance monitoring
- **Pre-flight Validation**: Comprehensive system checks before startup

### Enhanced Features
- **Colored Console Logging**: Beautiful, informative console output with spinners
- **Graceful Shutdown**: Proper cleanup and shutdown handling
- **Configuration Validation**: Comprehensive config file validation
- **Error Recovery**: Robust error handling and recovery mechanisms
- **Performance Monitoring**: CPU, memory, and disk usage tracking

## 🚀 Performance Features

### Optimized Single-Process Architecture
Meiko uses a single-process architecture optimized for SDR monitoring workloads. While pre-forking could improve web performance, it conflicts with SDRTrunk's single-instance requirement.

**Why Single-Process:**
- **SDRTrunk Compatibility** - Prevents multiple SDRTrunk instances competing for audio hardware
- **Resource Efficiency** - Avoids duplicate file watchers and database connections
- **Deterministic Behavior** - Ensures consistent audio processing and transcription
- **Simplified Debugging** - Single process makes troubleshooting easier

**Performance Optimizations:**
- Memory usage reduction enabled
- Optimized connection handling (256K max concurrent)
- Smart idle timeout management (60s)
- Keep-alive connections for faster response times
- Efficient static file serving with compression

**Future Scaling Options:**
- **Reverse Proxy** - Use nginx/Apache for static file serving and load balancing
- **CDN Integration** - Serve static assets from content delivery networks  
- **Database Optimization** - Connection pooling and query optimization
- **Caching Layer** - Redis/Memcached for frequently accessed data
- **Microservices** - Separate web dashboard from SDR processing if needed

## Installation

### Prerequisites

1. **Go 1.21 or later**
2. **Java Runtime Environment** (for SDRTrunk)
3. **Python 3.8+** (for local transcription)
4. **SDRTrunk** application
5. **faster-whisper** (for local transcription): `pip install faster-whisper`

### Build from Source

```bash
git clone https://github.com/your-username/Meiko.git
cd Meiko
go mod tidy
go build -o meiko
```

## Configuration

Copy and customize the configuration file:

```bash
cp config.yaml config-local.yaml
```

### Key Configuration Sections

#### SDRTrunk Settings
```yaml
sdrtrunk:
  # For JAR distribution
  path: "/path/to/sdrtrunk.jar"
  java_path: "java"
  jvm_args: ["-Xmx2g", "-Xms512m"]
  
  # OR for Linux binary distribution
  path: "/path/to/sdr-trunk"
  # java_path and jvm_args are ignored for binaries
  
  audio_output_dir: "/path/to/recordings"
```

#### Transcription Settings
```yaml
transcription:
  mode: "local"  # or "remote"
  local:
    whisper_script: "./fasterWhisper.py"
    python_path: "python"
    model_size: "tiny"
    device: "cpu"
    language: "en"
```

#### Discord Integration
```yaml
discord:
  token: "YOUR_DISCORD_BOT_TOKEN"
  channel_id: "YOUR_CHANNEL_ID"
  notifications:
    startup: true
    shutdown: true
    errors: true
    transcriptions: true
```

## Usage

### Basic Usage

1. **Configure the application**:
   ```bash
   # Edit config.yaml with your settings
   nano config.yaml
   ```

2. **Run Meiko**:
   ```bash
   ./meiko
   ```

### Command Line Options

```bash
# Use custom config file
./meiko -config custom-config.yaml

# Enable debug logging
./meiko -debug

# Show version
./meiko -version
```

### Pre-flight Checks

Meiko automatically runs pre-flight checks on startup:
- ✅ SDRTrunk path validation
- ✅ Java runtime availability
- ✅ Audio output directory permissions
- ✅ Transcription service configuration
- ✅ Database connectivity
- ✅ USB device detection (optional)

## Architecture

### Component Overview

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   SDRTrunk      │    │   File Watcher   │    │  Transcription  │
│   Manager       │───▶│                  │───▶│   Service       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Process       │    │   Call           │    │   Database      │
│   Monitor       │    │   Processor      │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Discord       │    │   System         │    │   Logger        │
│   Client        │    │   Monitor        │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### Data Flow

1. **SDRTrunk** generates audio recordings
2. **File Watcher** detects new files
3. **Call Processor** extracts metadata and queues for transcription
4. **Transcription Service** processes audio (local or remote)
5. **Database** stores call records and transcriptions
6. **Discord Client** sends notifications
7. **System Monitor** tracks performance and health

## Transcription Modes

### Local Mode (faster-whisper)

Uses the included `fasterWhisper.py` script with the faster-whisper library:

```yaml
transcription:
  mode: "local"
  local:
    whisper_script: "./fasterWhisper.py"
    model_size: "tiny"  # tiny, base, small, medium, large
    device: "cpu"       # cpu, cuda
```

**Advantages:**
- No internet required
- Lower latency
- Privacy-focused
- No API costs

### Remote Mode

Sends audio files to a remote transcription API:

```yaml
transcription:
  mode: "remote"
  remote:
    endpoint: "https://your-api.com/transcribe"
    api_key: "your-api-key"
    timeout: 30
```

**Advantages:**
- More powerful models
- No local compute requirements
- Centralized processing

## Discord Integration

### Bot Setup

1. Create a Discord application at https://discord.com/developers/applications
2. Create a bot and copy the token
3. Invite the bot to your server with appropriate permissions
4. Configure the bot token and channel ID in `config.yaml`

### Notification Types

- 🚀 **Startup/Shutdown**: Application lifecycle events
- ❌ **Errors**: Critical errors and failures
- 📞 **Transcriptions**: New call transcriptions
- 📊 **System Health**: Performance alerts and warnings

## Database Schema

### Calls Table
```sql
CREATE TABLE calls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL,
    filepath TEXT NOT NULL UNIQUE,
    talkgroup_id TEXT,
    from_id TEXT,
    to_id TEXT,
    unixtime INTEGER,
    duration REAL,
    transcription TEXT,
    processed BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## Monitoring and Logging

### Log Levels
- **DEBUG**: Detailed debugging information
- **INFO**: General information messages
- **WARN**: Warning messages
- **ERROR**: Error messages

### System Monitoring
- CPU usage monitoring
- Memory usage tracking
- Disk space monitoring
- Process health checks
- Automatic alerting

## Troubleshooting

### Common Issues

#### SDRTrunk Won't Start
```bash
# Check Java installation
java -version

# Verify SDRTrunk path
ls -la /path/to/sdrtrunk.jar

# Check permissions
chmod +x /path/to/sdrtrunk.jar
```

#### Transcription Failures
```bash
# Test Python and faster-whisper
python -c "import faster_whisper; print('OK')"

# Test whisper script directly
python ./fasterWhisper.py test-audio.mp3
```

#### Discord Connection Issues
- Verify bot token is correct
- Check bot permissions in Discord server
- Ensure channel ID is valid

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
logging:
  level: "DEBUG"
  colors: true
  timestamps: true
```

## Performance Tuning

### Transcription Performance
- Use GPU acceleration: `device: "cuda"`
- Adjust model size: `tiny` (fastest) to `large` (most accurate)
- Tune batch processing: `batch_size: 5`

### System Performance
- Adjust file monitoring interval: `poll_interval: 1000`
- Configure database connection pool: `max_open_conns: 10`
- Set appropriate JVM memory: `jvm_args: ["-Xmx4g"]`

## Development

### Project Structure
```
Meiko/
├── main.go                 # Application entry point
├── config.yaml            # Configuration file
├── fasterWhisper.py       # Transcription script
├── internal/              # Internal packages
│   ├── config/           # Configuration management
│   ├── database/         # Database operations
│   ├── discord/          # Discord integration
│   ├── logger/           # Logging system
│   ├── monitoring/       # System monitoring
│   ├── preflight/        # Pre-flight checks
│   ├── processor/        # Call processing
│   ├── sdrtrunk/         # SDRTrunk management
│   ├── transcription/    # Transcription services
│   └── watcher/          # File system monitoring
└── references/           # Reference implementations
```

### Building

```bash
# Development build
go build -o meiko

# Production build with optimizations
go build -ldflags "-s -w" -o meiko

# Cross-compilation for Linux
GOOS=linux GOARCH=amd64 go build -o meiko-linux

# Cross-compilation for Windows
GOOS=windows GOARCH=amd64 go build -o meiko.exe
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- **SDRTrunk**: The excellent SDR trunking application
- **faster-whisper**: High-performance speech recognition
- **Original Projects**: Swimtrunks and SdrTrunk-Transcriber for inspiration

## Support

- 📖 **Documentation**: Check this README and inline code comments
- 🐛 **Issues**: Report bugs via GitHub Issues
- 💬 **Discussions**: Use GitHub Discussions for questions

---

**Made with ❤️ for the radio monitoring community**
