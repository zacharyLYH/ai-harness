# .env
OPENROUTER_API_KEY

# Prerequisites
1. Get an [Openrouter API key](https://openrouter.ai/workspaces/default/keys). Create a `.env` file and paste the key inside. and paste it in the `.env` file under `OPENROUTER_API_KEY=`
2. Download latest Go version if you don't already have Go downloaded
3. `go mod tidy` to install dependencies
4. To run the project: `go run .` 
5. Test it out: `Explain the theory of relativity in caveman language.`

# Features
1. Basic agentic loop with read/write file abilities
2. Consent request for tool calls if NeedUserConsent = true
3. Skills
4. Loading indicator in chat session
5. Basic web search using duckduckgo api

# Developer features
1. Session logging into `common/logger/log.txt`

# Test it out
Paste the prompts into the terminal. You are encouraged to open up `common/logger/log.txt` to see the llm calls in action.
## Writing data agentically
1. `Do i have a file called user_data.csv? If I don't create a file user_data.csv and insert some mock data.` (We expect the bash tool)
2. Assuming you've done(1), `Find out if 'Xavier' is a person in the csv you created. If not, create it.` (We expect the bash tool)
## Use skill
1. `Write me a poem about a fat cat with a brown hat.` (We expect to use the poem skill)
## Using web search and multi-turn
1. `Find out how many people stay in lombok indonesia then write the list to a file` (We expect to use duckduckgo search tool and the write file tool)
2. It should declare it has 2 checklist items to do and will spawn up 2 subagents
3. Keep answering the terminal prompts for web search and write file