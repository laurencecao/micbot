#!/bin/bash

# 1. launch sglang server for baichuan LLM
python -m sglang.launch_server --tp 1 --model-path Baichuan-M2-32B-GPTQ-Int4 --reasoning-parser qwen3 --kv-cache-dtype fp8_e4m3 --attention-backend flashinfer --max-prefill-tokens 64000
#--max-total-tokens 32000

# 2. launch http api server for baichuan service
python main.py

#SGLANG_ALLOW_OVERWRITE_LONGER_CONTEXT_LEN=1 python3 -m sglang.launch_server \
#       --model Baichuan-M2-32B-GPTQ-Int4 \
#       --speculative-algorithm EAGLE3 \
#       --speculative-draft-model-path Baichuan-M2-32B-GPTQ-Int4/draft \
#       --speculative-num-steps 6 \
#       --speculative-eagle-topk 10 \
#       --speculative-num-draft-tokens 32 \
#       --mem-fraction 0.9 \
#       --cuda-graph-max-bs 2 \
#       --reasoning-parser qwen3 \
#       --quantization gptq
        #--dtype bfloat16