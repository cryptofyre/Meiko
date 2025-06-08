# Quick Start: Local Transcription Setup

## For Raspberry Pi 5 (Recommended)

```bash
# 1. Clone or copy Meiko files to your Pi
git clone https://github.com/your-repo/Meiko.git
cd Meiko

# 2. Run automated setup
./setup_pi_transcription.sh

# 3. Update config.yaml
nano config.yaml
# Change python_path to: ./venv/bin/python3

# 4. Test installation
python3 test_transcription.py

# 5. Run Meiko
./meiko
```

## For Other Linux Systems

```bash
# Install dependencies
sudo apt install python3 python3-pip python3-venv

# Setup Python environment
python3 -m venv venv
source venv/bin/activate
pip install faster-whisper

# Update config.yaml
transcription:
  mode: "local"
  local:
    python_path: "./venv/bin/python3"
    model_size: "tiny"
```

## Model Sizes

- **tiny** (32MB): Fastest, good quality ✅ **Recommended for Pi**
- **base** (74MB): Slower, better quality
- **small** (244MB): Slowest, best quality ❌ **Too big for Pi**

## First Run

The first transcription will download the model (~32MB for tiny). This is normal and only happens once.

---

**That's it!** Your local transcription is ready. 🎙️ 