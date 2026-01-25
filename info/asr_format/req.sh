#!/bin/bash

curl -X POST "http://localhost:8800/transcribe" -F "file=@demo.wav"

'
{"detected_language": "zh", "transcript": "...", "raw_segments": "..."}
'