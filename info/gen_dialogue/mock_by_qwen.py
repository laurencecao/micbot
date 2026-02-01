import os
import sys
import re
import shutil
import random
import requests
import dashscope
from pydub import AudioSegment
from dashscope.api_entities.dashscope_response import HTTPStatus

# 配置 API URL (根据你的 demo)
dashscope.base_http_api_url = 'https://dashscope.aliyuncs.com/api/v1'

# 角色与音色配置
ROLE_VOICE_MAP = {
    "全科医生": "Ethan",
    "患者": "Serena"
}
TEMP_DIR = "temp_audio_segments"

def clean_and_create_temp_dir():
    if os.path.exists(TEMP_DIR):
        shutil.rmtree(TEMP_DIR)
    os.makedirs(TEMP_DIR)

def parse_dialogue(file_path):
    """解析文本，提取 (角色, 内容) 列表"""
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()

    # 简单的按行解析，处理多行对话
    dialogues = []
    current_role = None
    buffer = []

    lines = content.split('\n')
    for line in lines:
        line = line.strip()
        if not line:
            continue

        # 检测角色
        match = re.match(r'^(全科医生|患者)[:：](.*)', line)
        if match:
            # 如果之前有缓存的内容，先保存
            if current_role and buffer:
                dialogues.append((current_role, "\n".join(buffer)))
                buffer = []
            
            current_role = match.group(1)
            text_content = match.group(2).strip()
            if text_content:
                buffer.append(text_content)
        else:
            # 延续上一行的内容
            if current_role:
                buffer.append(line)
    
    # 保存最后一段
    if current_role and buffer:
        dialogues.append((current_role, "\n".join(buffer)))
        
    return dialogues

def generate_tts(text, voice, index):
    """调用 DashScope API 生成语音并下载"""
    print(f"[{index}] 正在生成 ({voice}): {text[:20]}...")
    
    try:
        response = dashscope.MultiModalConversation.call(
            model="qwen3-tts-flash", # 根据 demo 使用的模型
            api_key=os.getenv("DASHSCOPE_API_KEY"),
            text=text,
            voice=voice,
            # language_type="Chinese" # 根据需要可开启
        )

        if response.status_code == HTTPStatus.OK:
            if response.output.audio and response.output.audio.url:
                audio_url = response.output.audio.url
                # 下载音频
                audio_data = requests.get(audio_url).content
                file_path = os.path.join(TEMP_DIR, f"{index:03d}_{voice}.wav")
                with open(file_path, 'wb') as f:
                    f.write(audio_data)
                return file_path
            else:
                print(f"API返回无音频数据: {response}")
        else:
            print(f"API调用失败: {response.code} - {response.message}")
            
    except Exception as e:
        print(f"发生异常: {e}")
    
    return None

def merge_audio_files(file_paths, output_file="final_dialogue.mp3"):
    """合并音频，加入随机静音"""
    combined = AudioSegment.empty()
    
    print("正在合并音频...")
    for i, file_path in enumerate(file_paths):
        if not file_path: continue
        
        segment = AudioSegment.from_wav(file_path)
        combined += segment
        
        # 如果不是最后一段，添加随机间隙 (0.5秒 到 1.5秒)
        if i < len(file_paths) - 1:
            silence_duration = random.randint(500, 1500) 
            combined += AudioSegment.silent(duration=silence_duration)

    combined.export(output_file, format="mp3")
    print(f"完成！文件已保存为: {output_file}")

def main():
    if len(sys.argv) < 2:
        print("用法: python script.py <文本文件路径>")
        sys.exit(1)

    input_file = sys.argv[1]
    
    # 1. 清理临时目录
    clean_and_create_temp_dir()
    
    # 2. 解析文本
    dialogues = parse_dialogue(input_file)
    print(f"解析到 {len(dialogues)} 段对话。")
    
    # 3. 逐段生成语音
    generated_files = []
    for idx, (role, text) in enumerate(dialogues):
        voice = ROLE_VOICE_MAP.get(role)
        if voice:
            file_path = generate_tts(text, voice, idx)
            if file_path:
                generated_files.append(file_path)
        else:
            print(f"跳过未知角色: {role}")

    # 4. 合并音频
    if generated_files:
        merge_audio_files(generated_files, "output_dialogue.mp3")
        # 清理 (可选)
        shutil.rmtree(TEMP_DIR)
    else:
        print("没有生成任何音频。")

if __name__ == "__main__":
    main()
