import json

def format_speech_by_speaker(json_data):
    formatted_lines = []
    current_speaker = None
    current_text = []

    # 遍历 JSON 中所有的片段 (segments)
    for segment in json_data:
        # 遍历片段中的每一个词 (words)
        for word_info in segment.get('words', []):
            speaker = word_info['speaker']
            word = word_info['word']

            # 如果说话人发生变化，保存当前内容并开始新行
            if speaker != current_speaker:
                if current_speaker is not None:
                    formatted_lines.append(f"{current_speaker}: {''.join(current_text)}")
                
                current_speaker = speaker
                current_text = [word]
            else:
                # 说话人相同，继续追加文本
                current_text.append(word)

    # 处理最后一段
    if current_speaker is not None:
        formatted_lines.append(f"{current_speaker}: {''.join(current_text)}")

    return formatted_lines

# 测试代码
if __name__ == "__main__":
    # 加载你的 JSON 数据
    with open('demo_result.json', 'r', encoding='utf-8') as f:
        data = json.load(f)

    result = format_speech_by_speaker(data)

    # 打印结果
    for line in result:
        print(line)
