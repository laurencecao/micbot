from flask import Flask, request, jsonify
from werkzeug.utils import secure_filename
import os
from pathlib import Path
from asr_service import get_asr_service

app = Flask(__name__)

# 配置文件
UPLOAD_FOLDER = 'uploads'
ALLOWED_EXTENSIONS = {'wav', 'mp3', 'm4a', 'flac', 'ogg', 'mp4', 'm4a', 'aac'}
MAX_CONTENT_LENGTH = 100 * 1024 * 1024  # 100MB

# 确保上传目录存在
os.makedirs(UPLOAD_FOLDER, exist_ok=True)

app.config['UPLOAD_FOLDER'] = UPLOAD_FOLDER
app.config['MAX_CONTENT_LENGTH'] = MAX_CONTENT_LENGTH

def allowed_file(filename):
    """检查文件扩展名是否允许"""
    return '.' in filename and \
           filename.rsplit('.', 1)[1].lower() in ALLOWED_EXTENSIONS

# 初始化ASR服务
asr_service = get_asr_service()

@app.route('/transcribe', methods=['POST'])
def transcribe_audio():
    """
    API接口：上传语音文件进行转写
    
    支持两种方式：
    1. 文件上传（multipart/form-data）
    2. 直接字节流（application/octet-stream）
    
    查询参数：
    - format: 音频格式（默认为自动检测）
    - use_itn: 是否使用ITN（数字转换），默认为true
    - output_timestamp: 是否输出时间戳，默认为true
    - merge_vad: 是否合并VAD分段，默认为true
    """
    
    # 获取查询参数
    use_itn = request.args.get('use_itn', 'true').lower() == 'true'
    output_timestamp = request.args.get('output_timestamp', 'true').lower() == 'true'
    merge_vad = request.args.get('merge_vad', 'true').lower() == 'true'
    merge_length_s = int(request.args.get('merge_length_s', '15'))
    
    # 准备生成参数
    generate_kwargs = {
        "use_itn": use_itn,
        "output_timestamp": output_timestamp,
        "merge_vad": merge_vad,
        "merge_length_s": merge_length_s,
    }
    
    try:
        # 方式1：文件上传
        if 'file' in request.files:
            file = request.files['file']
            if file.filename == '':
                return jsonify({"error": "No selected file"}), 400
            
            if file and allowed_file(file.filename):
                filename = secure_filename(file.filename)
                file_path = os.path.join(app.config['UPLOAD_FOLDER'], filename)
                file.save(file_path)
                
                try:
                    # 执行转写
                    text = asr_service.transcribe_from_file(file_path, **generate_kwargs)
                    
                    # 清理上传的文件
                    os.unlink(file_path)
                    
                    return jsonify({
                        "success": True,
                        "text": text,
                        "filename": filename
                    })
                except Exception as e:
                    # 确保文件被清理
                    if os.path.exists(file_path):
                        os.unlink(file_path)
                    return jsonify({"error": f"Transcription failed: {str(e)}"}), 500
            
            return jsonify({"error": "File type not allowed"}), 400
        
        # 方式2：直接字节流
        elif request.content_type == 'application/octet-stream' or request.data:
            audio_bytes = request.data
            
            if not audio_bytes:
                return jsonify({"error": "No audio data provided"}), 400
            
            # 获取音频格式
            audio_format = request.args.get('format', 'wav')
            
            # 执行转写
            text = asr_service.transcribe_from_bytes(audio_bytes, audio_format, **generate_kwargs)
            
            return jsonify({
                "success": True,
                "text": text,
                "format": audio_format,
                "size_bytes": len(audio_bytes)
            })
        
        else:
            return jsonify({"error": "No audio data provided. Use file upload or direct byte stream."}), 400
    
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/health', methods=['GET'])
def health_check():
    """健康检查接口"""
    return jsonify({
        "status": "healthy",
        "service": "ASR Transcription Service",
        "device": asr_service.device
    })

@app.route('/')
def index():
    """根目录返回API使用说明"""
    return """
    <h1>ASR Transcription Service</h1>
    <p>API Endpoints:</p>
    <ul>
        <li><b>POST /transcribe</b> - 上传语音文件进行转写</li>
        <li><b>GET /health</b> - 服务健康检查</li>
    </ul>
    <p>上传方式:</p>
    <ol>
        <li>文件上传: multipart/form-data with 'file' field</li>
        <li>字节流: application/octet-stream with raw audio data</li>
    </ol>
    <p>支持的音频格式: wav, mp3, m4a, flac, ogg, mp4, aac</p>
    """

if __name__ == '__main__':
    # 启动服务
    print("Starting ASR Transcription API Server...")
    print(f"Upload folder: {UPLOAD_FOLDER}")
    print(f"Max file size: {MAX_CONTENT_LENGTH / (1024*1024)} MB")
    
    app.run(host='0.0.0.0', port=5000, debug=True)

