# ai-harness
A mini ai agent built with pluggable tool access.

# Requirements
1. Agentic capabilities - Multi-turn conversations
2. Tools - Python scripts that the agent can call and use to perform certain actions

## Tools
1. Read write
    - file name
    - line number (start and end)
2. Write file
    - file name
    - content
    - directory (hard code)
3. List file
    - directory (hard code)
4. Internet search
    - curl

# Test
1. Create a sample csv with user data with fields like name, age, and gender
2. Find the user data csv and return distinct ages like the csv
3. Add more user data to the user csv
4. Find sample ASEAN currencies online then create another table with the currency and the exchange rate to USD at this moment.

# Diagram
https://excalidraw.com/#room=47d52a5a2add9e029c0d,1xykZrJI1P5_bIAK4dZFUg

# .env
SERP_API
OPENROUTER_API_KEY
GEMINI_API_KEY