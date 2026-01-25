import os
from openai import OpenAI

# 初始化客户端
client = OpenAI(
    api_key="sk-no-key-required",
    base_url="http://localhost:30000/v1"
)

def read_file(file_path):
    if os.path.exists(file_path):
        with open(file_path, 'r', encoding='utf-8') as f:
            return f.read()
    return ""

def generate_soap():
    dialogue = read_file("input.txt")
    soap_tmpl = read_file("soap.tmpl")
    history = read_file("input2.txt")

    if not dialogue:
        print("错误：input.txt 不能为空。")
        return

    # 构造针对 Qwen2/Baichuan 的标准 Prompt 格式
    full_prompt = f"""<|im_start|>system
你是一位专业的临床医生。请根据背景信息和对话，整理出SOAP格式病历。<|im_end|>
<|im_start|>user
{soap_tmpl}

### 诊疗记录:
{history}

### 对话内容:
{dialogue}

请生成最终的 SOAP 记录：<|im_end|>
<|im_start|>assistant
"""

    try:
        # 方案 A: 使用 completions 接口 (对本地 SGLang 更稳健)
        response = client.completions.create(
            model="Baichuan-M2-32B-GPTQ-Int4",
            prompt=full_prompt,
            max_tokens=64000,
            temperature=0.1,
            stop=["<|im_end|>", "<|endoftext|>"]
        )
        
        result = response.choices[0].text.strip()

        # 如果方案 A 没拿到数据，尝试方案 B (Chat)
        if not result:
            print("Completions 接口未返回数据，尝试 Chat 接口...")
            chat_res = client.chat.completions.create(
                model="Baichuan-M2-32B-GPTQ-Int4",
                messages=[
                    {"role": "system", "content": "你是一位医生。"},
                    {"role": "user", "content": full_prompt}
                ]
            )
            result = chat_res.choices[0].message.content

        if result:
            print("=== 生成的 SOAP 诊疗记录 ===")
            print(result)
            with open("output_soap.txt", "w", encoding="utf-8") as f:
                f.write(result)
        else:
            print("错误：模型返回内容为空，请检查 SGLang 服务端日志。")

    except Exception as e:
        print(f"请求发生异常: {str(e)}")

if __name__ == "__main__":
    generate_soap()

