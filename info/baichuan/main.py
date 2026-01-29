import os
import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from openai import OpenAI
from dotenv import load_dotenv

# --- 0. 加载环境变量 ---
load_dotenv()

# 配置变量 (支持默认值)
API_KEY = os.getenv("API_KEY", "sk-no-key-required")
BASE_URL = os.getenv("BASE_URL", "http://localhost:30000/v1")
MODEL_NAME = os.getenv("MODEL_NAME", "Baichuan-M2-32B-GPTQ-Int4")
HOST = os.getenv("HOST", "0.0.0.0")
PORT = int(os.getenv("PORT", 8000))

app = FastAPI()

# --- 1. 启动时加载模板 ---
TEMPLATE_PATH = "soap.tmpl"
if not os.path.exists(TEMPLATE_PATH):
    # 如果文件不存在，写入一个简单的默认模板防止启动失败
    with open(TEMPLATE_PATH, 'w', encoding='utf-8') as f:
        f.write("请生成SOAP病历") 
    # raise FileNotFoundError(f"找不到模板文件: {TEMPLATE_PATH}")

with open(TEMPLATE_PATH, 'r', encoding='utf-8') as f:
    SOAP_TEMPLATE = f.read()

# --- 2. 初始化 OpenAI 客户端 ---
client = OpenAI(
    api_key=API_KEY,
    base_url=BASE_URL
)

# --- 3. 定义请求数据模型 ---
class SOAPRequest(BaseModel):
    dialogue: str
    history: str = ""  # 默认为空

# --- 4. API 路由 ---
@app.post("/generate_soap")
async def generate_soap_api(req: SOAPRequest):
    if not req.dialogue:
        raise HTTPException(status_code=400, detail="dialogue 不能为空")

    # 构造 Prompt
    full_prompt = f"""<|im_start|>system
你是一位专业的临床医生。请根据背景信息和对话，整理出SOAP格式病历。注意由于听写原故，部分的字可能同音但是错字，请按你的医学常识修正它们。<|im_end|>
<|im_start|>user
{SOAP_TEMPLATE}

### 诊疗记录:
{req.history}

### 对话内容:
{req.dialogue}

请生成最终的 SOAP 记录：<|im_end|>
<|im_start|>assistant
"""

    try:
        result = None
        # 优先尝试 Completions 接口 (本地模型常用)
        # response = client.completions.create(
        #     model=MODEL_NAME,
        #     prompt=full_prompt,
        #     max_tokens=4096,
        #     temperature=0.1,
        #     stop=["<|im_end|>", "<|endoftext|>"]
        # )
        
        # result = response.choices[0].text.strip()

        # 兜底逻辑：Chat 接口 (部分在线API可能不支持 completions)
        if not result:
            # 注意：如果切换到完全不支持raw prompt的在线API，这里可能需要调整 message 构造
            chat_res = client.chat.completions.create(
                model=MODEL_NAME,
                messages=[
                    {"role": "system", "content": "你是一位专业的临床医生"},
                    {"role": "user", "content": full_prompt} 
                ]
            )
            result = chat_res.choices[0].message.content
            print("invoke baichuan done!")
        return {"status": "success", "data": result}

    except Exception as e:
        # 记录错误日志
        print(f"Error: {e}")
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    uvicorn.run(app, host=HOST, port=PORT)

