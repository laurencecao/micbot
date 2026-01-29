#!/bin/bash

curl -X POST "http://localhost:8000/generate_soap" \
     -H "Content-Type: application/json" \
     -d '{
           "dialogue": "Patient complains of chest pain...",
           "history": "No prior history."
         }'