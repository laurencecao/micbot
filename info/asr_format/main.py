import os
import torch
import whisperx
import gc
from fastapi import FastAPI, UploadFile, File, HTTPException, Form
# from whisperx.diarize import DiarizationPipeline
import shutil
import tempfile
from typing import Optional
import requests
import json
from http import HTTPStatus
from dashscope.audio.asr import Transcription
from urllib import request
from urllib.parse import urlparse
import dashscope
import oss2
import time
import uuid

app = FastAPI()

# --- Configuration ---
DEVICE = "cuda" if torch.cuda.is_available() else "cpu"
COMPUTE_TYPE = "float16" if DEVICE == "cuda" else "int8"
BATCH_SIZE = 16
HF_TOKEN = os.environ.get('HF_TOKEN')
TOKEN_302 = os.environ.get('TOKEN_302')
API_302_URL = os.environ.get('API_302_URL')

dashscope.base_http_api_url = 'https://dashscope.aliyuncs.com/api/v1'
dashscope.api_key = os.environ.get("DASHSCOPE_API_KEY")

OSS_ACCESS_KEY_ID = os.environ.get('OSS_ACCESS_KEY_ID')
OSS_ACCESS_KEY_SECRET = os.environ.get('OSS_ACCESS_KEY_SECRET')
OSS_ENDPOINT = os.environ.get('OSS_ENDPOINT', 'https://oss-cn-shanghai.aliyuncs.com')
OSS_BUCKET_NAME = os.environ.get('OSS_BUCKET_NAME')

print(f"using OSS_ACCESS_KEY_ID:{OSS_ACCESS_KEY_ID}")
print(f"using OSS_ACCESS_KEY_SECRET:{OSS_ACCESS_KEY_SECRET}")
print(f"using OSS_ENDPOINT:{OSS_ENDPOINT}")
print(f"using OSS_BUCKET_NAME:{OSS_BUCKET_NAME}")
# Global Models
print(f"Loading models on {DEVICE}...")
# MODEL = whisperx.load_model("large-v3-turbo", DEVICE, compute_type=COMPUTE_TYPE)
# DIARIZE_MODEL = DiarizationPipeline(use_auth_token=HF_TOKEN, device=DEVICE)
MODEL = None
DIARIZE_MODEL = None


def using_online_api(tmp_path, language=None):   
    # url = "https://api.302ai.cn/302/whisperx"
    url = API_302_URL
    
    # Configuration based on screenshot
    payload = {
        "processing_type": "diarize", # Essential for speaker labels
        "translate": "false",
        "output": "json"              # Must be json to support segment parsing
    }
    
    # Add language only if specified
    if language:
        payload["language"] = language
    # File upload
    files = [
        ('audio_input', (os.path.basename(tmp_path), open(tmp_path, 'rb'), 'application/octet-stream'))
    ]
    
    headers = {
        'Authorization': f'Bearer {TOKEN_302}'
    }
    
    # Use standard POST request
    response = requests.post(url, headers=headers, data=payload, files=files)
    
    # Return the parsed dictionary directly
    ret = response.json()

    with open('/tmp/resp_302ai.json', 'w') as wfl:
        wfl.write(json.dumps(ret))

    return ret

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

@app.post("/transcribe3")
async def transcribe_audio(file: UploadFile = File(...)):
    # 1. Prepare temp file
    with tempfile.NamedTemporaryFile(delete=False, suffix=".wav") as tmp:
        shutil.copyfileobj(file.file, tmp)
        tmp_path = tmp.name
    try:
        # 2. Get result from Online API
        # response.json() in the helper function already returns a Dict
        result = using_online_api(tmp_path)
        #print("......", result, "......")
        
        # REMOVED: inner_json_str = json.loads(api_response_raw) 
        # REMOVED: result = json.loads(inner_json_str)
        # 4. Extract Segments and Language
        segments = result.get("segments", [])
        language = result.get("language", "zh") # Default to en if missing
        
        # 5. Format Output
        formatted_list = format_speech_by_speaker(segments)
        
        # 6. Return consistent structure
        ret = {
            "detected_language": language,
            "transcript": "\n".join(formatted_list),
            "raw_segments": segments
        }
        with open('/tmp/dbg.json', 'w') as wfl:
            wfl.write(json.dumps(ret))
        return ret
    except Exception as e:
        import traceback
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))
    finally:
        # Cleanup
        if os.path.exists(tmp_path):
            os.remove(tmp_path)

@app.post("/transcribe2")
async def transcribe_audio2(
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


def upload_to_oss(local_file_path, object_name=None):
    auth = oss2.Auth(OSS_ACCESS_KEY_ID, OSS_ACCESS_KEY_SECRET)
    bucket = oss2.Bucket(auth, OSS_ENDPOINT, OSS_BUCKET_NAME)
    if object_name is None:
        ext = os.path.splitext(local_file_path)[1]
        object_name = f"asr/{uuid.uuid4()}{ext}"
    # Retry mechanism configuration
    max_retries = 3
    
    for attempt in range(max_retries):
        try:
            print(f"upload {local_file_path} to oss {object_name} (Attempt {attempt + 1})")
            bucket.put_object_from_file(object_name, local_file_path)
            break  # Success: break out of the retry loop
        except Exception as e:
            print(f"Upload error: {e}")
            if attempt == max_retries - 1:
                raise e  # Failed all attempts: re-raise the exception
            time.sleep(1)  # Wait 1 second before retrying
    https_url = bucket.sign_url('GET', object_name, expires=10)
    print(f"save to oss: {https_url}")
    return https_url


def format_funasr_transcript(sentences):
    formatted_lines = []
    for sentence in sentences:
        speaker_id = sentence.get('speaker_id', 0)
        text = sentence.get('text', '').strip()
        if text:
            formatted_lines.append(f"speaker {speaker_id:02d}:{text}")
    return formatted_lines


@app.post("/transcribe")
async def transcribe_audio_funasr(file: UploadFile = File(...)):
    with tempfile.NamedTemporaryFile(delete=False, suffix=".wav") as tmp:
        shutil.copyfileobj(file.file, tmp)
        tmp_path = tmp.name

    oss_url = None
    try:
        oss_url = upload_to_oss(tmp_path)

        # 添加重试机制，因为 Transcription.async_call 偶尔会因网络问题失败
        max_retries = 3
        retry_delay = 2  # 秒
        task_response = None
        last_error = None

        for attempt in range(max_retries):
            try:
                task_response = Transcription.async_call(
                    model='fun-asr',
                    file_urls=[oss_url],
                    diarization_enabled=True,
                    language_hints=['zh', 'en']
                )
                break  # 成功则跳出循环
            except Exception as e:
                last_error = e
                print(f"Transcription.async_call attempt {attempt + 1}/{max_retries} failed: {e}")
                if attempt < max_retries - 1:
                    print(f"Retrying in {retry_delay} seconds...")
                    time.sleep(retry_delay)
                else:
                    print("All retries exhausted.")

        if task_response is None:
            raise last_error if last_error else Exception("Failed to call Transcription.async_call")

        transcription_response = Transcription.wait(task=task_response.output.task_id)

        if transcription_response.status_code != HTTPStatus.OK:
            raise HTTPException(
                status_code=500,
                detail=f"FunASR API error: {transcription_response.output.message}"
            )

        all_sentences = []
        detected_language = "zh"

        for transcription in transcription_response.output['results']:
            if transcription['subtask_status'] == 'SUCCEEDED':
                url = transcription['transcription_url']
                result = json.loads(request.urlopen(url).read().decode('utf8'))

                for transcript_data in result.get('transcripts', []):
                    sentences = transcript_data.get('sentences', [])
                    all_sentences.extend(sentences)
            else:
                print(f"Transcription failed: {transcription}")

        formatted_list = format_funasr_transcript(all_sentences)

        ret = {
            "detected_language": detected_language,
            "transcript": "\n".join(formatted_list),
            "raw_segments": all_sentences
        }
        with open('/tmp/result.json', 'w') as wfl:
            wfl.write(json.dumps(ret))
        return ret

    except Exception as e:
        import traceback
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))
    finally:
        if os.path.exists(tmp_path):
            os.remove(tmp_path)
        if oss_url:
            try:
                auth = oss2.Auth(OSS_ACCESS_KEY_ID, OSS_ACCESS_KEY_SECRET)
                bucket = oss2.Bucket(auth, OSS_ENDPOINT, OSS_BUCKET_NAME)
                parsed = urlparse(oss_url)
                object_name = parsed.path.lstrip('/')
                bucket.delete_object(object_name)
            except Exception as e:
                print(f"Failed to delete OSS object: {e}")


if __name__ == "__main__":
    import uvicorn
    print(f"using BACKEND: {API_302_URL}")
    uvicorn.run(app, host="0.0.0.0", port=8800)

