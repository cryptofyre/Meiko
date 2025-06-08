#!/usr/bin/env python3
"""
FasterWhisper Transcription Module for Meiko
Optimized for Raspberry Pi 5 with minimal resource usage
"""

import argparse
import json
import sys
import os
import logging
import time
from pathlib import Path
from typing import Optional, Dict, Any

try:
    from faster_whisper import WhisperModel
except ImportError:
    print(json.dumps({"error": "faster-whisper not installed. Run: pip install faster-whisper"}), file=sys.stderr)
    sys.exit(1)

# Configure logging
logging.basicConfig(
    level=logging.WARNING,  # Only show warnings/errors to keep output clean
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

class FasterWhisperTranscriber:
    """
    Optimized Faster-Whisper transcriber for embedded systems
    """
    
    def __init__(self, 
                 model_size: str = "tiny",
                 device: str = "cpu",
                 language: Optional[str] = "en",
                 compute_type: str = "int8"):
        """
        Initialize the transcriber with optimized settings for Raspberry Pi 5
        
        Args:
            model_size: Model size (tiny, base, small) - tiny recommended for Pi
            device: Device to use (cpu only on Pi 5)
            language: Language code for transcription (None for auto-detect)
            compute_type: Quantization type for efficiency (int8 for Pi)
        """
        self.model_size = model_size
        self.device = device
        self.language = language if language and language != "auto" else None
        self.compute_type = compute_type
        self.model = None
        
        # Raspberry Pi optimizations
        self.cpu_threads = min(4, os.cpu_count() or 4)  # Pi 5 has 4 cores
        
    def load_model(self) -> None:
        """Load the Whisper model with optimizations for Pi 5"""
        try:
            logger.info(f"Loading Whisper model: {self.model_size}")
            start_time = time.time()
            
            self.model = WhisperModel(
                self.model_size,
                device=self.device,
                compute_type=self.compute_type,
                cpu_threads=self.cpu_threads,
                # Download to a persistent cache directory
                download_root=os.path.expanduser("~/.cache/faster-whisper"),
                # Use less memory by not loading unnecessary components
                local_files_only=False
            )
            
            load_time = time.time() - start_time
            logger.info(f"Model loaded in {load_time:.2f}s")
            
        except Exception as e:
            logger.error(f"Failed to load model: {e}")
            raise
    
    def transcribe_file(self, audio_path: str) -> Dict[str, Any]:
        """
        Transcribe an audio file and return results
        
        Args:
            audio_path: Path to the audio file
            
        Returns:
            Dictionary with transcription results
        """
        if not os.path.exists(audio_path):
            raise FileNotFoundError(f"Audio file not found: {audio_path}")
        
        # Load model if not already loaded
        if self.model is None:
            self.load_model()
        
        try:
            logger.info(f"Transcribing: {os.path.basename(audio_path)}")
            start_time = time.time()
            
            # Transcribe with optimized settings for speed/efficiency
            segments, info = self.model.transcribe(
                audio_path,
                language=self.language,
                # Speed optimizations
                beam_size=1,              # Faster than default 5
                best_of=1,                # Faster than default 5
                patience=1.0,             # Less patience for faster results
                # Quality vs speed tradeoffs
                temperature=0.0,          # Deterministic output
                condition_on_previous_text=False,  # Faster processing
                # VAD settings for better silence detection
                vad_filter=True,
                vad_parameters=dict(
                    min_silence_duration_ms=500,
                    threshold=0.5,
                    speech_pad_ms=400
                ),
                # Memory optimizations
                word_timestamps=False,    # Disable to save memory/time
                without_timestamps=True   # Text only, no timing info needed
            )
            
            # Combine all segments into single text
            full_text = ""
            segment_count = 0
            
            for segment in segments:
                full_text += segment.text + " "
                segment_count += 1
            
            # Clean up the text
            full_text = full_text.strip()
            
            transcription_time = time.time() - start_time
            
            # Prepare result
            result = {
                "text": full_text,
                "language": info.language if hasattr(info, 'language') else self.language,
                "duration": transcription_time,
                "segments": segment_count,
                "model_size": self.model_size,
                "file_size_mb": round(os.path.getsize(audio_path) / (1024 * 1024), 2)
            }
            
            logger.info(f"Transcription completed in {transcription_time:.2f}s")
            logger.info(f"Text length: {len(full_text)} chars, Segments: {segment_count}")
            
            return result
            
        except Exception as e:
            logger.error(f"Transcription failed: {e}")
            raise

def validate_audio_file(file_path: str) -> bool:
    """Validate that the audio file exists and is readable"""
    if not os.path.exists(file_path):
        return False
    
    # Check file size (should be at least 1KB)
    if os.path.getsize(file_path) < 1024:
        return False
    
    # Check file extension
    valid_extensions = {'.mp3', '.wav', '.m4a', '.ogg', '.flac', '.aac'}
    file_ext = Path(file_path).suffix.lower()
    
    return file_ext in valid_extensions

def main():
    """Main entry point for the transcription script"""
    parser = argparse.ArgumentParser(
        description="FasterWhisper transcription for Meiko (Raspberry Pi optimized)"
    )
    parser.add_argument("audio_file", help="Path to audio file to transcribe")
    parser.add_argument("--model", default="tiny", 
                       choices=["tiny", "base", "small"],
                       help="Model size (default: tiny for Pi 5)")
    parser.add_argument("--language", default="en",
                       help="Language code (default: en)")
    parser.add_argument("--device", default="cpu",
                       help="Device to use (default: cpu)")
    parser.add_argument("--verbose", action="store_true",
                       help="Enable verbose logging")
    
    args = parser.parse_args()
    
    # Set logging level
    if args.verbose:
        logging.getLogger().setLevel(logging.INFO)
    
    try:
        # Validate input file
        if not validate_audio_file(args.audio_file):
            result = {
                "error": f"Invalid or missing audio file: {args.audio_file}"
            }
            print(json.dumps(result))
            sys.exit(1)
        
        # Initialize transcriber
        transcriber = FasterWhisperTranscriber(
            model_size=args.model,
            device=args.device,
            language=args.language if args.language != "auto" else None
        )
        
        # Perform transcription
        result = transcriber.transcribe_file(args.audio_file)
        
        # Output JSON result to stdout (as expected by Meiko)
        print(json.dumps(result, ensure_ascii=False))
        
    except KeyboardInterrupt:
        result = {"error": "Transcription cancelled by user"}
        print(json.dumps(result))
        sys.exit(1)
        
    except Exception as e:
        result = {"error": f"Transcription failed: {str(e)}"}
        print(json.dumps(result))
        sys.exit(1)

if __name__ == "__main__":
    main() 