# AI Harness Tools

Agents need tools to interact with your machine. Instead of giving it unrestricted bash access, we create scoped capabilities for our agents. This directory contains the tools the agent can use.

## Structure

Each tool is a Python file with JSON metadata at the top (inside triple-quoted comments), followed by the function implementation.

```python
"""
{
    "Name": "NameOfToolNoSpaces",
    "Description": "Description of tool in detail",
    "Params": {
        "Properties": [
            {
                "param1": {
                    "type": "string",
                    "description": "description of this param1"
                }
            }
        ],
        "Required": ["param1"]
    }
}
"""
def NameOfToolNoSpaces(param1: type):
    # function body
```

## Safety

Every tool requires user permission before execution. The permission system uses a `(toolName, directory)` pair:

- Filesystem tools (read_file, write_file, list_file, etc.) accept a `directory` parameter.
- When the LLM calls a tool, the agent asks for permission: "Tool 'read_file' in directory '/path' wants to run. Allow? (y/N)"
- Once approved, the `(toolName, directory)` pair is cached for the session. Subsequent calls to the same tool in the same directory are auto-approved.
- Web tools (curl_web, duckduckgo_search) have no directory parameter — permission is granted per tool name only.

This means the agent can operate in any directory, but only after you explicitly grant permission for each tool+directory combination.