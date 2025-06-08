# Meiko Local Transcription Setup

## Overview

Meiko supports local transcription using OpenAI's Whisper model via the `faster-whisper` library. This setup is optimized for **Raspberry Pi 5** but works on any Linux system.

## 🍓 Raspberry Pi 5 Optimizations

The included `fasterWhisper.py` script is specifically optimized for Raspberry Pi 5:

- **Tiny Model**: Uses the smallest Whisper model (32MB) for fast performance
- **CPU Optimized**: Uses INT8 quantization for efficient CPU processing
- **Memory Efficient**: Minimal memory footprint with optimized settings
- **Fast Startup**: Model caching reduces initialization time

## 📦 Quick Setup (Raspberry Pi)

1. **Clone Meiko to your Raspberry Pi 5**:
   ```bash
   git clone https://github.com/your-repo/Meiko.git
   cd Meiko
   ```

2. **Run the automated setup**:
   ```bash
   ./setup_pi_transcription.sh
   ```

3. **Update your config.yaml**:
   ```yaml
   transcription:
     mode: "local"
     local:
       whisper_script: "./fasterWhisper.py"
       python_path: "./venv/bin/python3"  # Use the virtual environment
       model_size: "tiny"                  # Recommended for Pi 5
       device: "cpu"
       language: "en"
   ```

4. **Test the setup**:
   ```bash
   python3 test_transcription.py
   ```

## 🔧 Manual Setup

If you prefer manual installation:

### Prerequisites

```bash
# Update system
sudo apt update

# Install dependencies
sudo apt install -y python3 python3-pip python3-venv python3-dev \
                    build-essential libffi-dev libssl-dev \
                    libsndfile1-dev portaudio19-dev
```

### Python Environment

```bash
# Create virtual environment
python3 -m venv venv
source venv/bin/activate

# Install PyTorch (CPU-only for efficiency)
pip install torch torchaudio --index-url https://download.pytorch.org/whl/cpu

# Install faster-whisper
pip install faster-whisper>=1.0.0
```

### Configuration

Update your `config.yaml`:

```yaml
transcription:
  mode: "local"
  local:
    whisper_script: "./fasterWhisper.py"
    python_path: "./venv/bin/python3"
    model_size: "tiny"
    device: "cpu"
    language: "en"
```

## 🎯 Model Performance Comparison

| Model | Size   | Speed (Pi 5) | Accuracy | Memory | Recommended |
|-------|--------|--------------|----------|--------|-------------|
| tiny  | 32MB   | ~2-3x RT     | Good     | ~300MB | ✅ **Yes**  |
| base  | 74MB   | ~1-2x RT     | Better   | ~500MB | ⚠️ Maybe    |
| small | 244MB  | ~0.5x RT     | Best     | ~1GB   | ❌ No       |

*RT = Real Time (1x = same speed as audio duration)*

## 🧪 Testing

### Basic Test
```bash
# Test script functionality
python3 fasterWhisper.py --help

# Test with audio file
python3 fasterWhisper.py your_audio.mp3
```

### Comprehensive Test
```bash
# Run full test suite
python3 test_transcription.py
```

### Manual Test with Sample Audio
```bash
# Create test audio (if espeak available)
espeak "Testing Meiko transcription system" -w test.wav

# Transcribe
python3 fasterWhisper.py test.wav --verbose

# Expected output:
# {"text": "Testing Meiko transcription system", "language": "en", ...}
```

## 🎛️ Advanced Configuration

### Custom Model Settings

You can customize the transcription behavior by editing `fasterWhisper.py`:

```python
# In the transcribe_file method, adjust these parameters:
segments, info = self.model.transcribe(
    audio_path,
    language=self.language,
    beam_size=1,              # 1=fastest, 5=better quality
    best_of=1,                # 1=fastest, 5=better quality
    patience=1.0,             # 1.0=faster, 2.0=more thorough
    temperature=0.0,          # 0.0=deterministic, 0.2=more creative
    vad_filter=True,          # Voice activity detection
    vad_parameters=dict(
        min_silence_duration_ms=500,  # Minimum silence to detect
        threshold=0.5,                # Voice detection sensitivity
    ),
)
```

### Environment Variables

You can also configure via environment variables:

```bash
export WHISPER_MODEL_SIZE=tiny
export WHISPER_LANGUAGE=en
export WHISPER_DEVICE=cpu
python3 fasterWhisper.py audio.mp3
```

## 🚀 Performance Tips

### Raspberry Pi 5 Specific

1. **Enable GPU Memory Split** (optional):
   ```bash
   sudo raspi-config
   # Advanced Options > Memory Split > 128
   ```

2. **Increase Swap** (if getting memory errors):
   ```bash
   sudo dphys-swapfile swapoff
   sudo nano /etc/dphys-swapfile
   # Set CONF_SWAPSIZE=2048
   sudo dphys-swapfile setup
   sudo dphys-swapfile swapon
   ```

3. **CPU Governor**:
   ```bash
   # Set performance mode for faster transcription
   echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor
   ```

### General Optimization

- **Use SSD**: Store models and temp files on SSD for faster access
- **Cool the Pi**: Ensure good ventilation to avoid thermal throttling
- **Batch Processing**: Process multiple files together when possible

## 🔍 Troubleshooting

### Common Issues

**"faster-whisper not installed"**
```bash
pip install faster-whisper
```

**"No module named 'torch'"**
```bash
pip install torch torchaudio --index-url https://download.pytorch.org/whl/cpu
```

**"Model download failed"**
```bash
# Check internet connection and try manual download
python3 -c "from faster_whisper import WhisperModel; WhisperModel('tiny')"
```

**"Transcription too slow"**
- Use "tiny" model instead of "base" or "small"
- Reduce beam_size and best_of parameters
- Enable CPU performance governor
- Check for thermal throttling

**"Out of memory errors"**
- Increase swap space
- Use "tiny" model
- Close other applications
- Restart the Pi

### Debug Mode

Run with verbose logging:
```bash
python3 fasterWhisper.py audio.mp3 --verbose
```

Check Meiko logs:
```bash
# Meiko will log transcription attempts
tail -f meiko.log | grep -i transcription
```

## 📊 Expected Performance

On Raspberry Pi 5 with tiny model:
- **Loading time**: ~3-5 seconds (first run)
- **Transcription speed**: 2-3x real-time
- **Memory usage**: ~300-400MB
- **Accuracy**: Good for clear audio, excellent for radio traffic

## 🔄 Integration with Meiko

Once configured, Meiko will:

1. **Detect new audio files** from SDRTrunk
2. **Queue for transcription** using the local service
3. **Call fasterWhisper.py** with the audio file path
4. **Parse JSON output** and store in database
5. **Update web dashboard** with transcriptions
6. **Send Discord notifications** (if configured)

## 📋 File Structure

```
Meiko/
├── fasterWhisper.py              # Main transcription script
├── requirements.txt              # Python dependencies
├── setup_pi_transcription.sh     # Automated setup script
├── test_transcription.py         # Test utilities
├── config.yaml                   # Main configuration
└── venv/                         # Python virtual environment (after setup)
    └── bin/python3               # Python interpreter to use
```

## 🆘 Support

If you encounter issues:

1. **Check the logs**: Look for error messages in Meiko output
2. **Test manually**: Try running `fasterWhisper.py` directly
3. **Verify setup**: Run `test_transcription.py`
4. **Check resources**: Monitor CPU/memory usage during transcription
5. **Report issues**: Include system info, error messages, and sample audio

---

**Ready to transcribe!** 🎙️ Your Raspberry Pi 5 is now equipped with local, fast, and efficient speech-to-text capabilities. 