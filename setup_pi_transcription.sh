#!/bin/bash
# Setup script for Meiko FasterWhisper on Raspberry Pi 5
# This script installs the necessary dependencies for local transcription

set -e  # Exit on any error

echo "🍓 Setting up Meiko FasterWhisper for Raspberry Pi 5"
echo "=================================================="

# Check if running on Raspberry Pi
if ! grep -q "Raspberry Pi" /proc/cpuinfo 2>/dev/null; then
    echo "⚠️  Warning: This script is optimized for Raspberry Pi, but will continue anyway."
fi

# Update system packages
echo "📦 Updating system packages..."
sudo apt update

# Install system dependencies
echo "🔧 Installing system dependencies..."
sudo apt install -y \
    python3 \
    python3-pip \
    python3-venv \
    python3-dev \
    build-essential \
    libffi-dev \
    libssl-dev \
    libbz2-dev \
    libreadline-dev \
    libsqlite3-dev \
    libncurses5-dev \
    libncursesw5-dev \
    xz-utils \
    tk-dev \
    libfftw3-dev \
    libsndfile1-dev \
    portaudio19-dev

# Create virtual environment
echo "🐍 Creating Python virtual environment..."
if [ ! -d "venv" ]; then
    python3 -m venv venv
fi

# Activate virtual environment
echo "🔌 Activating virtual environment..."
source venv/bin/activate

# Upgrade pip
echo "⬆️  Upgrading pip..."
pip install --upgrade pip setuptools wheel

# Install PyTorch CPU-only (smaller, faster for Pi)
echo "🧠 Installing PyTorch (CPU-only)..."
pip install torch torchaudio --index-url https://download.pytorch.org/whl/cpu

# Install faster-whisper and dependencies
echo "🎤 Installing faster-whisper..."
pip install -r requirements.txt

# Make the script executable
echo "🔐 Making fasterWhisper.py executable..."
chmod +x fasterWhisper.py

# Test the installation
echo "🧪 Testing installation..."
python3 fasterWhisper.py --help > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✅ Installation successful!"
    echo ""
    echo "📝 Usage:"
    echo "  # Activate the virtual environment:"
    echo "  source venv/bin/activate"
    echo ""
    echo "  # Test transcription (replace with actual audio file):"
    echo "  python3 fasterWhisper.py test.mp3"
    echo ""
    echo "  # Use with Meiko (update config.yaml):"
    echo "  transcription:"
    echo "    mode: local"
    echo "    local:"
    echo "      whisper_script: ./fasterWhisper.py"
    echo "      python_path: ./venv/bin/python3"
    echo "      model_size: tiny"
    echo ""
    echo "🚀 Ready to use with Meiko!"
else
    echo "❌ Installation test failed. Please check the output above for errors."
    exit 1
fi

# Show memory usage tip
echo ""
echo "💡 Raspberry Pi 5 Tips:"
echo "  - Use 'tiny' model for fastest performance (32MB)"
echo "  - Use 'base' model for better accuracy (74MB)"
echo "  - Consider increasing swap if you get memory errors"
echo "  - First run will download the model (~32MB for tiny)"

# Show model download info
echo ""
echo "📥 Model Download Info:"
echo "  Models are cached in: ~/.cache/faster-whisper"
echo "  tiny model: ~32MB"
echo "  base model: ~74MB"
echo "  small model: ~244MB (not recommended for Pi)"

echo ""
echo "🎉 Setup complete! Deactivating virtual environment."
deactivate 