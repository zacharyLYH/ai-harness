# Quick start
Download the binary from here.
```bash
curl -sSL https://raw.githubusercontent.com/zacharyLYH/ai-harness/main/install.sh | sh
```
`install.sh` detects your OS/arch, downloads the matching asset from `releases/latest/download`, and installs the binary to `/usr/local/bin`.

You will also need to get an [Openrouter API key](https://openrouter.ai/workspaces/default/keys)

# Features
1. Bash access that requires consent before execution
2. General consent request for tool calls
3. Add, view, update, delete skills
4. Checklists and subagents, to keep the model in line and focused on the overall goal
5. Basic web search using duckduckgo api

# Test it out
Paste the prompts into the terminal. You are encouraged to open up `common/logger/log.txt` to see the llm calls in action.
## Writing data agentically
1. `Do i have a file called user_data.csv? If I don't create a file user_data.csv and insert some mock data.` (We expect the bash tool)
2. Assuming you've done(1), `Find out if 'Xavier' is a person in the csv you created. If not, create it.` (We expect the bash tool)
## Use skill
1. `Write me a poem about a fat cat with a brown hat.` (We expect to use the poem skill)

## Manage skills
Skills are reusable instruction files stored in `~/.ai-harness/skills`. Manage them from the prompt:

```text
/skill list
/skill create
/skill show <name>
/skill edit <name>
/skill delete <name>
```

`/skill create` and `/skill edit` guide you through the name, short trigger description, and Markdown instructions. New and edited skills are available in the next message; use `/skill show <name>` to review exactly what will be used. `/skills`, `/skill add`, and `/skill view` remain supported aliases.

## Using web search and multi-turn
1. `Find out how many people stay in lombok indonesia then write the list to a file` (We expect to use duckduckgo search tool and the write file tool)
2. It should declare it has 2 checklist items to do and will spawn up 2 subagents
3. Keep answering the terminal prompts for web search and write file

# Development

## .env
OPENROUTER_API_KEY

## Prerequisites
1. Get an [Openrouter API key](https://openrouter.ai/workspaces/default/keys). Create a `.env` file and paste the key inside. and paste it in the `.env` file under `OPENROUTER_API_KEY=`
2. Download latest Go version if you don't already have Go downloaded
3. `go mod tidy` to install dependencies
4. To run the project: `go run .` 
5. Test it out: `Explain the theory of relativity in caveman language.`

## How to deploy
Every push to `main` runs the CI workflow (`go build`, `go vet`, `go test`). That validates the code but does **not** publish binaries. To make a downloadable release others can install, you also need to cut a version tag:

1. Make sure `main` is green (the checks job passed on your last push).
2. Tag a release and push the tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
   The `release` job only runs on `v*` tags, runs `./build.sh` to cross-compile, and uploads the `dist/*.tar.gz` assets to the GitHub release.
3. Verify the release on GitHub shows the five `ai-harness-<os>-<arch>.tar.gz` assets.

That's it — the tag is the extra step beyond the automatic per-push build.

## Developer quality of life features
1. Session logging into `common/logger/log.txt` and `common/logger/raw_log.txt`.
