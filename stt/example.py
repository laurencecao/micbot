import requests
import time
from pathlib import Path

def example_file_upload():
    """示例：通过文件上传进行转写"""
    url = "http://localhost:5000/transcribe"
    
    # 上传文件
    with open('/path/to/your/audio.mp3', 'rb') as f:
        files = {'file': f}
        
        # 可选参数通过查询字符串传递
        params = {
            'use_itn': 'true',
            'merge_vad': 'true',
            'merge_length_s': '15'
        }
        
        response = requests.post(url, files=files, params=params)
        
        if response.status_code == 200:
            result = response.json()
            print("转写成功!")
            print(f"文本长度: {len(result['text'])}")
            print(f"转写结果: {result['text'][:200]}...")  # 显示前200个字符
        else:
            print(f"错误: {response.json()}")

def example_direct_bytes():
    """示例：直接发送字节流进行转写"""
    url = "http://localhost:5000/transcribe"
    
    # 读取音频文件为字节流
    with open('/path/to/your/audio.wav', 'rb') as f:
        audio_bytes = f.read()
    
    # 设置请求头
    headers = {'Content-Type': 'application/octet-stream'}
    
    # 查询参数
    params = {
        'format': 'wav',
        'use_itn': 'true'
    }
    
    response = requests.post(url, data=audio_bytes, headers=headers, params=params)
    
    if response.status_code == 200:
        result = response.json()
        print("转写成功!")
        print(f"转写结果: {result['text']}")
    else:
        print(f"错误: {response.json()}")

def example_local_usage():
    """示例：本地直接使用ASR服务"""
    from asr_service import ASRService
    
    # 创建服务实例
    service = ASRService()
    
    # 方式1: 从文件转写
    text = service.transcribe_from_file(
        '/path/to/your/audio.wav',
        use_itn=True,
        merge_vad=True
    )
    print(f"文件转写结果: {text}")
    
    # 方式2: 从字节流转写
    with open('/path/to/your/audio.mp3', 'rb') as f:
        audio_bytes = f.read()
    
    text = service.transcribe_from_bytes(
        audio_bytes,
        audio_format='mp3',
        use_itn=True
    )
    print(f"字节流转写结果: {text}")

if __name__ == '__main__':
    # 等待服务启动
    time.sleep(2)
    
    # 运行示例
    print("=== 示例: 本地使用 ===")
    example_local_usage()
    
    print("\n=== 示例: HTTP API文件上传 ===")
    example_file_upload()
    
    print("\n=== 示例: HTTP API字节流 ===")
    example_direct_bytes()

