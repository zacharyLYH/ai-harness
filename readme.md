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
OPENROUTER_API_KEY

# Usage
1. Download latest Go version and get the required API keys [here](https://openrouter.ai/workspaces/default/keys). Create a `.env` file and paste the key inside.
2. `cd` to root directory of this project
3. `go mod tidy` to install dependencies
4. To run the project: `go run .` Test out a casual prompt like `Explain the theory of relativity in caveman language.`
5. Let's test out agentic capabilities. The llm is constrained to `/llm_directory` as a safety sandbox. Try out this prompt: `Do i have a file called user_data.csv?`. It should say this file is not present. Without killing the session, create a file `user_data.csv` inside `/llm_directory` and reprompt the question. It should say it is present.
6. Check out `common/logger/log.txt` for the system logs

# Skills test
1. Write me a poem according my requirements 