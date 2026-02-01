import os
import re
import json
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
# 加载 soap.tmpl (用于 generate_soap)
TEMPLATE_PATH = "soap.tmpl"
if not os.path.exists(TEMPLATE_PATH):
    # 如果文件不存在，写入一个简单的默认模板防止启动失败
    with open(TEMPLATE_PATH, 'w', encoding='utf-8') as f:
        f.write("请生成SOAP病历")
    # raise FileNotFoundError(f"找不到模板文件: {TEMPLATE_PATH}")

with open(TEMPLATE_PATH, 'r', encoding='utf-8') as f:
    SOAP_TEMPLATE = f.read()

# 加载 soap2.tmpl (用于 generate_medical_record)
TEMPLATE_PATH_2 = "soap2.tmpl"
if not os.path.exists(TEMPLATE_PATH_2):
    # 如果文件不存在，写入一个简单的默认模板防止启动失败
    with open(TEMPLATE_PATH_2, 'w', encoding='utf-8') as f:
        f.write("请生成HIS诊疗记录")

with open(TEMPLATE_PATH_2, 'r', encoding='utf-8') as f:
    SOAP2_TEMPLATE = f.read()


# --- 1.5 解析 HIS 记录为结构化数据 ---
def parse_his_record(text: str) -> dict:
    """将 HIS 记录文本解析为结构化数据，便于表格化呈现"""
    # 定义字段映射
    fields = [
        ("主诉", r"1\.\s*主诉[:：]\s*(.*?)(?=2\.\s*现病史|$)"),
        ("现病史", r"2\.\s*现病史[:：]\s*(.*?)(?=3\.\s*既往史|$)"),
        ("既往史", r"3\.\s*既往史[:：]\s*(.*?)(?=4\.\s*药物过敏史|$)"),
        ("药物过敏史", r"4\.\s*药物过敏史[:：]\s*(.*?)(?=5\.\s*体格检查|$)"),
        ("体格检查", r"5\.\s*体格检查[:：]\s*(.*?)(?=6\.\s*辅助检查|$)"),
        ("辅助检查", r"6\.\s*辅助检查[^：]*[:：]\s*(.*?)(?=7\.\s*诊断|$)"),
        ("诊断", r"7\.\s*诊断[:：]\s*(.*?)(?=8\.\s*处理|$)"),
        ("处理", r"8\.\s*处理[:：]\s*(.*?)(?=9\.\s*注意事项|$)"),
        ("注意事项", r"9\.\s*注意事项[:：]\s*(.*?)(?=10\.\s*健康宣教|$)"),
        ("健康宣教", r"10\.\s*健康宣教[:：]\s*(.*?)(?=11\.\s*医师签名|$)"),
        ("医师签名", r"11\.\s*医师签名[:：]\s*(.*?)$"),
    ]

    result = {}
    for field_name, pattern in fields:
        match = re.search(pattern, text, re.DOTALL | re.IGNORECASE)
        if match:
            value = match.group(1).strip()
            # 清理多余空白
            value = re.sub(r'\n\s*', '\n', value)
            result[field_name] = value if value else "暂无"
        else:
            result[field_name] = "暂无"

    return result


# --- 2. 初始化 OpenAI 客户端 ---
client = OpenAI(
    api_key=API_KEY,
    base_url=BASE_URL
)

# --- 3. 定义请求数据模型 ---
class SOAPRequest(BaseModel):
    dialogue: str
    history: str = ""  # 默认为空


class MedicalRecordRequest(BaseModel):
    dialogue: str
    history: str = ""  # 默认为空，对应检查记录

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


@app.post("/generate_medical_record")
async def generate_medical_record_api(req: MedicalRecordRequest):
    if not req.dialogue:
        raise HTTPException(status_code=400, detail="dialogue 不能为空")

    # 构造 Prompt - 要求直接返回 JSON
    full_prompt = f"""<|im_start|>system
你是一位专业的临床医生。请根据医患对话和检查记录，整理出专业的HIS诊疗记录。
注意由于听写原故，部分的字可能有错字，请按你的医学常识纠正它们，但不要自己主观臆测或添加任何不在对话中的信息,更不要错漏任何已经明确提及的信息，要非常精准。
语言简练、医学术语准确。如果对话中未提及某项，请标记为“暂无”或根据历史记录补充。必须严格区分医生和病人的对话逻辑。
你必须直接返回一个有效的JSON对象，不要包含任何其他文本、markdown代码块标记或解释。JSON格式如下：
{{
    "主诉": "...",
    "现病史": "...",
    "既往史": "...",
    "药物过敏史": "...",
    "体格检查": "...",
    "辅助检查（与本次疾病相关）": "...",
    "诊断": "...",
    "处理": "...",
    "注意事项": "...",
    "健康宣教": "...",
    "医师签名": "..."
}}
如果某项未提及，填写"暂无"。 
<|im_start|>user

### 检查记录:
{req.history}

### 医患对话:
{req.dialogue}

请直接返回JSON格式的HIS诊疗记录： 
<|im_start|>assistant
"""

    try:
        chat_res = client.chat.completions.create(
            model=MODEL_NAME,
            messages=[
                {"role": "system", "content": "你是一位专业的临床医生，必须直接返回JSON格式，不要添加任何markdown标记或其他文本"},
                {"role": "user", "content": full_prompt}
            ],
            temperature=0.01,
        )
        result = chat_res.choices[0].message.content
        print("invoke baichuan done!")
        print(f"Raw result: {result[:200]}...")

        # 尝试直接解析 JSON
        table_data = None
        try:
            # 清理可能的 markdown 代码块标记
            cleaned = result.strip()
            if cleaned.startswith("```json"):
                cleaned = cleaned[7:]
            elif cleaned.startswith("```"):
                cleaned = cleaned[3:]
            if cleaned.endswith("```"):
                cleaned = cleaned[:-3]
            cleaned = cleaned.strip()

            table_data = json.loads(cleaned)
            print(f"Successfully parsed JSON: {list(table_data.keys())}")
        except json.JSONDecodeError as e:
            print(f"JSON parse failed: {e}, falling back to regex parsing")
            # 回退到正则解析
            table_data = parse_his_record(result)

        return {
            "status": "success",
            "data": result,
            "table_data": table_data
        }

    except Exception as e:
        # 记录错误日志
        print(f"Error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


if __name__ == "__main__":
    uvicorn.run(app, host=HOST, port=PORT)

