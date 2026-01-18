import torch
import tempfile
import os
from pathlib import Path
from typing import Dict, Any, Optional, Union
from funasr import AutoModel


class ASRService:
    """语音转写服务类"""
    
    def __init__(self, 
                 model_dir: str = "FunAudioLLM/Fun-ASR-Nano-2512",
                 vad_model: str = "fsmn-vad",
                 device: Optional[str] = None):
        """
        初始化ASR服务
        
        Args:
            model_dir: 模型目录或名称
            vad_model: VAD模型名称
            device: 设备类型，None为自动选择
        """
        if device is None:
            device = (
                "cuda:0"
                if torch.cuda.is_available()
                else "mps"
                if torch.backends.mps.is_available()
                else "cpu"
            )
        
        self.device = device
        self.model_dir = model_dir
        self.vad_model = vad_model
        
        print(f"Loading ASR model on device: {device}")
        self.model = AutoModel(
            model=model_dir,
            trust_remote_code=True,
            vad_model=vad_model,
            vad_kwargs={"max_single_segment_time": 30000},
            remote_code="./model.py",
            device=device,
        )
        print("ASR model loaded successfully")
    
    def transcribe_from_bytes(self, 
                             audio_bytes: bytes,
                             audio_format: str = "wav",
                             **kwargs) -> str:
        """
        从字节流转写语音
        
        Args:
            audio_bytes: 音频字节流
            audio_format: 音频格式 (wav, mp3等)
            **kwargs: 动态参数传递给generate方法
            
        Returns:
            转写后的文本
        """
        # 创建临时文件保存音频
        with tempfile.NamedTemporaryFile(suffix=f'.{audio_format}', delete=False) as tmp_file:
            tmp_file.write(audio_bytes)
            tmp_path = tmp_file.name
        
        try:
            # 使用默认参数，允许通过kwargs覆盖
            default_kwargs = {
                "cache": {},
                "use_itn": True,
                "batch_size": 1,
                "batch_size_s": 0,
                "merge_vad": True,
                "merge_length_s": 15,
                "ban_emo_unk": True,
                "output_timestamp": True
            }
            
            # 合并默认参数和传入参数
            generate_kwargs = {**default_kwargs, **kwargs}
            
            # 执行转写
            res = self.model.generate(
                input=[tmp_path],
                **generate_kwargs
            )
            
            # 提取文本
            if res and len(res) > 0:
                text = res[0].get("text", "")
                return text
            else:
                return ""
        finally:
            # 清理临时文件
            try:
                os.unlink(tmp_path)
            except:
                pass
    
    def transcribe_from_file(self, 
                            file_path: Union[str, Path],
                            **kwargs) -> str:
        """
        从文件转写语音
        
        Args:
            file_path: 音频文件路径
            **kwargs: 动态参数传递给generate方法
            
        Returns:
            转写后的文本
        """
        with open(file_path, 'rb') as f:
            audio_bytes = f.read()
        
        # 根据文件扩展名确定格式
        ext = Path(file_path).suffix.lstrip('.').lower()
        if ext in ['mp3', 'm4a', 'flac', 'ogg']:
            audio_format = ext
        else:
            audio_format = 'wav'  # 默认
            
        return self.transcribe_from_bytes(audio_bytes, audio_format, **kwargs)


# 全局服务实例
_service_instance = None

def get_asr_service(**init_kwargs) -> ASRService:
    """获取或创建ASR服务实例（单例模式）"""
    global _service_instance
    if _service_instance is None:
        _service_instance = ASRService(**init_kwargs)
    return _service_instance

