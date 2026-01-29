import os
import torch
import whisperx
import gc
from fastapi import FastAPI, UploadFile, File, HTTPException, Form
from whisperx.diarize import DiarizationPipeline
import shutil
import tempfile
from typing import Optional

app = FastAPI()

# --- Configuration ---
DEVICE = "cuda" if torch.cuda.is_available() else "cpu"
COMPUTE_TYPE = "float16" if DEVICE == "cuda" else "int8"
BATCH_SIZE = 16
HF_TOKEN = os.environ['HF_TOKEN']

# Global Models
print(f"Loading models on {DEVICE}...")
MODEL = whisperx.load_model("large-v3-turbo", DEVICE, compute_type=COMPUTE_TYPE)
DIARIZE_MODEL = DiarizationPipeline(use_auth_token=HF_TOKEN, device=DEVICE)

def format_speech_by_speaker(segments):
    formatted_lines = []
    current_speaker = None
    current_text = []

    for segment in segments:
        # Check if word-level speaker info exists
        words = segment.get('words', [])
        if not words:
            # Fallback to segment-level speaker if words aren't aligned
            speaker = segment.get('speaker', "UNKNOWN")
            text = segment.get('text', '').strip()
            formatted_lines.append(f"{speaker}: {text}")
            continue

        for word_info in words:
            # Note: WhisperX sometimes uses 'speaker' in word_info after assignment
            speaker = word_info.get('speaker', current_speaker or "UNKNOWN")
            word = word_info.get('word', "")

            if speaker != current_speaker:
                if current_speaker is not None:
                    formatted_lines.append(f"{current_speaker}: {''.join(current_text)}")
                current_speaker = speaker
                current_text = [word]
            else:
                current_text.append(word)

    if current_speaker is not None:
        formatted_lines.append(f"{current_speaker}: {''.join(current_text)}")
    
    return formatted_lines

@app.post("/transcribe")
async def transcribe_audio(
    file: UploadFile = File(...),
    language: Optional[str] = Form(None) # Allow manual language override
):
    with tempfile.NamedTemporaryFile(delete=False, suffix=".wav") as tmp:
        shutil.copyfileobj(file.file, tmp)
        tmp_path = tmp.name

    try:
        # 1. Transcribe
        audio = whisperx.load_audio(tmp_path)
        result = MODEL.transcribe(audio, batch_size=BATCH_SIZE)
        
        # FIX: Robust language detection
        detected_language = language or result.get("language")
        if not detected_language:
             # Default to English if detection fails
            detected_language = "en" 

        # 2. Align
        try:
            model_a, metadata = whisperx.load_align_model(
                language_code=detected_language, device=DEVICE
            )
            result = whisperx.align(
                result["segments"], model_a, metadata, audio, DEVICE, return_char_alignments=False
            )
            # Cleanup alignment model immediately
            del model_a
            gc.collect()
            torch.cuda.empty_cache()
        except Exception as e:
            print(f"Alignment failed: {e}, skipping alignment step.")

        # 3. Diarize
        diarize_segments = DIARIZE_MODEL(audio)
        
        # 4. Assign speakers
        # Important: assign_word_speakers adds 'speaker' key to words in result["segments"]
        result = whisperx.assign_word_speakers(diarize_segments, result)

        # 5. Format Output
        formatted_list = format_speech_by_speaker(result["segments"])
        
        return {
            "detected_language": detected_language,
            "transcript": "\n".join(formatted_list),
            "raw_segments": result["segments"]
        }

    except Exception as e:
        import traceback
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))
    finally:
        if os.path.exists(tmp_path):
            os.remove(tmp_path)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8800)

