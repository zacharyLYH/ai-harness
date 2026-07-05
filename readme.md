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
1. `Do i have a file called user_data.csv? If I don't create a file user_data.csv and insert some mock data.` (We expect the read and write file tool)
2. Assuming you've done(1), `Use the grep tool to find out if 'Xavier' is a person in the csv you created. If not, create it.` (We expect the grepper and write file tool)
3. `Write me a poem about a fat cat with a brown hat.` (We expect to use the poem skill)