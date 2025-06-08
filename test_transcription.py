#!/usr/bin/env python3
"""
Test script for Meiko FasterWhisper transcription
"""

import json
import os
import sys
import tempfile
import subprocess
from pathlib import Path

def create_test_audio():
    """Create a simple test audio file using system text-to-speech (if available)"""
    test_text = "This is a test of the Meiko transcription system."
    
    # Try to create a test audio file
    # This is platform-dependent and may not work everywhere
    try:
        # On Linux with espeak
        if os.system("which espeak > /dev/null 2>&1") == 0:
            test_file = "/tmp/test_audio.wav"
            os.system(f'espeak "{test_text}" -w {test_file}')
            if os.path.exists(test_file):
                return test_file, test_text
    except:
        pass
    
    return None, test_text

def test_transcription():
    """Test the transcription system"""
    print("🧪 Testing Meiko FasterWhisper Transcription")
    print("=" * 50)
    
    # Check if script exists
    script_path = Path("fasterWhisper.py")
    if not script_path.exists():
        print("❌ fasterWhisper.py not found!")
        return False
    
    print("✅ fasterWhisper.py found")
    
    # Test help command
    try:
        result = subprocess.run([
            sys.executable, "fasterWhisper.py", "--help"
        ], capture_output=True, text=True)
        
        if result.returncode == 0:
            print("✅ Script help command works")
        else:
            print("❌ Script help command failed")
            print("Error:", result.stderr)
            return False
    except Exception as e:
        print(f"❌ Failed to run script: {e}")
        return False
    
    # Try to create test audio
    print("\n🎵 Attempting to create test audio...")
    test_file, expected_text = create_test_audio()
    
    if test_file and os.path.exists(test_file):
        print(f"✅ Test audio created: {test_file}")
        print(f"📝 Expected text: '{expected_text}'")
        
        # Test transcription
        try:
            print("\n🎤 Running transcription test...")
            result = subprocess.run([
                sys.executable, "fasterWhisper.py", test_file, "--verbose"
            ], capture_output=True, text=True, timeout=60)
            
            if result.returncode == 0:
                try:
                    output = json.loads(result.stdout)
                    print("✅ Transcription successful!")
                    print(f"📄 Result: '{output.get('text', 'No text found')}'")
                    print(f"🌐 Language: {output.get('language', 'Unknown')}")
                    print(f"⏱️  Duration: {output.get('duration', 0):.2f}s")
                    print(f"📊 Model: {output.get('model_size', 'Unknown')}")
                    
                    # Cleanup
                    os.remove(test_file)
                    return True
                except json.JSONDecodeError:
                    print("❌ Invalid JSON output from transcription")
                    print("Stdout:", result.stdout)
                    print("Stderr:", result.stderr)
            else:
                print("❌ Transcription failed")
                print("Error:", result.stderr)
                print("Output:", result.stdout)
        
        except subprocess.TimeoutExpired:
            print("❌ Transcription timed out (>60s)")
        except Exception as e:
            print(f"❌ Transcription error: {e}")
        
        # Cleanup
        if os.path.exists(test_file):
            os.remove(test_file)
    
    else:
        print("⚠️  Cannot create test audio file")
        print("💡 To test manually:")
        print("   1. Get an audio file (MP3, WAV, etc.)")
        print("   2. Run: python3 fasterWhisper.py your_audio_file.mp3")
        print("   3. Check that JSON output contains 'text' field")
    
    print("\n📋 System Requirements Check:")
    
    # Check Python version
    python_version = sys.version_info
    if python_version >= (3, 8):
        print(f"✅ Python {python_version.major}.{python_version.minor} (OK)")
    else:
        print(f"❌ Python {python_version.major}.{python_version.minor} (Need 3.8+)")
    
    # Check faster-whisper import
    try:
        import faster_whisper
        print("✅ faster-whisper library available")
    except ImportError:
        print("❌ faster-whisper not installed")
        print("💡 Run: pip install faster-whisper")
    
    return False

def main():
    """Main test function"""
    if test_transcription():
        print("\n🎉 All tests passed! Ready for Raspberry Pi deployment.")
        print("\n📦 For Raspberry Pi 5 setup:")
        print("   1. Transfer files to Pi")
        print("   2. Run: ./setup_pi_transcription.sh")
        print("   3. Update config.yaml python_path to: ./venv/bin/python3")
        return 0
    else:
        print("\n❌ Some tests failed. Check the output above.")
        print("\n🔧 Troubleshooting:")
        print("   - Install faster-whisper: pip install faster-whisper")
        print("   - Check Python version (need 3.8+)")
        print("   - Test manually with an audio file")
        return 1

if __name__ == "__main__":
    sys.exit(main()) 