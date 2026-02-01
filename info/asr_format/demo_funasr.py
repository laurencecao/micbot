from http import HTTPStatus
from dashscope.audio.asr import Transcription
from urllib import request
import dashscope
import os
import json

# 以下为北京地域url，若使用新加坡地域的模型，需将url替换为：https://dashscope-intl.aliyuncs.com/api/v1
dashscope.base_http_api_url = 'https://dashscope.aliyuncs.com/api/v1'

# 新加坡和北京地域的API Key不同。获取API Key：https://help.aliyun.com/zh/model-studio/get-api-key
# 若没有配置环境变量，请用百炼API Key将下行替换为：dashscope.api_key = "sk-xxx"
dashscope.api_key = os.getenv("DASHSCOPE_API_KEY", "sk-6364947727a64ca19c30c30701e1e49e")

task_response = Transcription.async_call(
    model='fun-asr',
    file_urls=['https://oss-pai-rmia4s0uublyu39v4o-cn-shanghai.oss-cn-shanghai.aliyuncs.com/5.txt.mp3?Expires=1769933045&OSSAccessKeyId=TMP.3KonyaJhYXXgBQ1xhc2T1DQ9yvj9FNQzmdPGzwZsVbZTE195dLgCwPt2oWT16mXJRQNxboJ8wntYTqhCg3tQiKEv5ARJJc&Signature=j5U99664AfCpDz%2FpTM63E2tO2Gw%3D'],
    diarization_enabled=True, 
    language_hints=['zh', 'en']  # language_hints为可选参数，用于指定待识别音频的语言代码。取值范围请参见API参考文档。
)

transcription_response = Transcription.wait(task=task_response.output.task_id)

if transcription_response.status_code == HTTPStatus.OK:
    for transcription in transcription_response.output['results']:
        if transcription['subtask_status'] == 'SUCCEEDED':
            url = transcription['transcription_url']
            result = json.loads(request.urlopen(url).read().decode('utf8'))
            result1 = json.dumps(result, indent=4, ensure_ascii=False)
            with open('out.json', 'w') as wfl:
                wfl.write(result1)
            print(result1)
        else:
            print('transcription failed!')
            print(transcription)
else:
    print('Error: ', transcription_response.output.message)
