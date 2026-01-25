import os
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from openai import OpenAI

app = FastAPI()

# --- 1. 启动时加载模板 ---
TEMPLATE_PATH = "soap.tmpl"
if not os.path.exists(TEMPLATE_PATH):
    raise FileNotFoundError(f"找不到模板文件: {TEMPLATE_PATH}")

with open(TEMPLATE_PATH, 'r', encoding='utf-8') as f:
    SOAP_TEMPLATE = f.read()

# --- 2. 初始化 OpenAI 客户端 ---
client = OpenAI(
    api_key="sk-no-key-required",
    base_url="http://localhost:30000/v1"
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
        # 优先尝试 Completions 接口
        response = client.completions.create(
            model="Baichuan-M2-32B-GPTQ-Int4",
            prompt=full_prompt,
            max_tokens=4096, # 建议根据实际调整，64000过大可能导致OOM
            temperature=0.1,
            stop=["<|im_end|>", "<|endoftext|>"]
        )
        
        result = response.choices[0].text.strip()

        # 兜底逻辑：Chat 接口
        if not result:
            chat_res = client.chat.completions.create(
                model="Baichuan-M2-32B-GPTQ-Int4",
                messages=[
                    {"role": "system", "content": "你是一位专业的临床医生"},
                    {"role": "user", "content": full_prompt}
                ]
            )
            result = chat_res.choices[0].message.content

        return {"status": "success", "data": result}

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
