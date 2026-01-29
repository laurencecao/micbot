#!/bin/bash

CUDA_VISIBLE_DEVICES="1" python -m tools.api_server --listen 0.0.0.0:8080 --llama-checkpoint-path "fishaudio/openaudio-s1-mini" --decoder-checkpoint-path fishaudio/openaudio-s1-mini/codec.pth --decoder-config-name modded_dac_vq