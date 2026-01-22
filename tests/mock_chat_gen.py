import requests
import base64
import os
import shutil
import subprocess
import glob

# 配置项
API_URL = "http://localhost:8080/v1/tts"
INPUT_FILE = "input.txt"
OUTPUT_DIR = "outputs"
FINAL_OUTPUT = "final_output.wav"
FINAL_MP3 = "final_output.mp3"  # Add this

SPEAKERS = {
    "Speaker 0": {
        "audio_file": "0.wav",
        "ref_text": "如果大家想听到更丰富更及时的直播内容，记得在周一到周五准时进入直播间，和大家一起畅聊新消费新科技新趋势。"
    },
    "Speaker 1": {
        "audio_file": "1.wav",
        "ref_text": "周一到周五，每天早晨七点半到九点半的直播片段。言下之意呢，就是废话有点多，大家也别嫌弃，因为这都是直播间最真实的状态了。"
    }
}

def clean_and_prepare_dir():
    if os.path.exists(OUTPUT_DIR):
        shutil.rmtree(OUTPUT_DIR)
    os.makedirs(OUTPUT_DIR)

def create_silence(duration, output_path):
    """生成指定秒数的静音文件 (采样率需与TTS一致，通常44100或32000)"""
    try:
        subprocess.run([
            'ffmpeg', '-y', '-f', 'lavfi', '-i', 'anullsrc=r=44100:cl=mono', 
            '-t', str(duration), '-q:a', '9', output_path
        ], check=True, capture_output=True)
    except Exception as e:
        print(f"生成静音失败: {e}")

def file_to_base64(filepath):
    if not os.path.exists(filepath): return None
    with open(filepath, "rb") as f:
        return base64.b64encode(f.read()).decode('utf-8')

def tts_request(text, ref_audio_b64, ref_text, output_path):
    payload = {
        "text": text,
        "references": [{"audio": ref_audio_b64, "text": ref_text}],
        "format": "wav",
        "normalize": True
    }
    try:
        response = requests.post(API_URL, json=payload, timeout=60)
        if response.status_code == 200:
            with open(output_path, "wb") as f:
                f.write(response.content)
            return True
    except Exception as e:
        print(f"请求异常: {e}")
    return False

def merge_audio():
    # 注意：如果TTS输出和静音文件的采样率/通道不一致，-c copy 可能会失效
    # 建议使用重新编码以保证兼容性
    wav_files = sorted(glob.glob(os.path.join(OUTPUT_DIR, "*.wav")))
    if not wav_files: return

    list_path = os.path.join(OUTPUT_DIR, "list.txt")
    with open(list_path, "w", encoding="utf-8") as f:
        for wav in wav_files:
            f.write(f"file '{os.path.basename(wav)}'\n")

    print("正在合并音频（含停顿）...")
    try:
        # 这里移除 -c copy，改用重新编码以确保静音文件与语音完美融合
        subprocess.run([
            'ffmpeg', '-y', '-f', 'concat', '-safe', '0', 
            '-i', list_path, '-ar', '44100', '-ac', '1', FINAL_OUTPUT
        ], check=True, capture_output=True)
        print(f"合并完成: {FINAL_OUTPUT}")
    except subprocess.CalledProcessError as e:
        print(f"FFmpeg 失败: {e.stderr.decode()}")

def convert_to_mp3():
    if not os.path.exists(FINAL_OUTPUT):
        return
    print(f"正在转换为 MP3: {FINAL_MP3}...")
    try:
        subprocess.run([
            'ffmpeg', '-y', '-i', FINAL_OUTPUT, 
            '-codec:a', 'libmp3lame', '-q:a', '2', FINAL_MP3
        ], check=True, capture_output=True)
        print("转换完成！")
    except subprocess.CalledProcessError as e:
        print(f"转换 MP3 失败: {e.stderr.decode()}")

def main():
    clean_and_prepare_dir()
    
    for key in SPEAKERS:
        b64 = file_to_base64(SPEAKERS[key]["audio_file"])
        if b64: SPEAKERS[key]["b64"] = b64
        else: return

    if not os.path.exists(INPUT_FILE): return

    with open(INPUT_FILE, "r", encoding="utf-8") as f:
        lines = [line.strip() for line in f if line.strip()]

    last_speaker = None
    file_idx = 0

    for i, line in enumerate(lines):
        if ":" not in line: continue
        speaker_key, content = [x.strip() for x in line.split(":", 1)]
        
        if speaker_key not in SPEAKERS: continue

        # --- 插入停顿逻辑 ---
        if last_speaker is not None:
            # 不同说话人停顿0.6s，同说话人(通常是句号结尾)停顿0.5s
            duration = 0.6 if speaker_key != last_speaker else 0.5
            silence_name = os.path.join(OUTPUT_DIR, f"{str(file_idx).zfill(4)}_pause.wav")
            create_silence(duration, silence_name)
            file_idx += 1

        # --- 生成语音 ---
        output_name = os.path.join(OUTPUT_DIR, f"{str(file_idx).zfill(4)}_{speaker_key}.wav")
        print(f"处理 [{i+1}/{len(lines)}]: {speaker_key}")
        if tts_request(content, SPEAKERS[speaker_key]["b64"], SPEAKERS[speaker_key]["ref_text"], output_name):
            file_idx += 1
            last_speaker = speaker_key

    merge_audio()
    convert_to_mp3()

if __name__ == "__main__":
    main()
