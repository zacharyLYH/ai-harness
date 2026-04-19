Agents need tools to interact with your machine. Instead of giving it unrestricted bash access, we will be creating scoped out capabilities to our agents. This is a directory of tools that this agent will be able to use.

Ideally, every time we add a tool here, we shouldn't need to change code anywhere else so that python tools that end up here are always self documenting. Since we want to enable self discovery of all tools, the structure of each file is very important.

# Structure
We will write metadata about the file as a JSON blob at the top of the function as multi line comments. Then below it the actual function itself. 

> At runtime, the code will parse all tools in this directory, extract metadata on its own and use them inside prompts.

```
NameOfToolNoSpaces.py

"""
{
    "Name": "NameOfToolNoSpaces",
    "Description": "Description of tool in detail",
    "Params": {
        "Properties": [
            {
                "param1": {
                    "type": "string, int, bool, etc",
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

# Safety
For safety purposes, we will be constraining the tool execution to the `/llm_directory` directory - all read, write files happen in here. We will do this by **hard coding** into the tools the directory that the llm can work in.