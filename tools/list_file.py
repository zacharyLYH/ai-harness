"""
{
    "Name": "list_file",
    "Description": "Lists files in the llm_directory using the ls command",
    "Params": {
        "Properties": [
            {
                "file_name": {
                    "type": "string",
                    "description": "The name of the file to look for (can be pattern or wildcard)"
                }
            }
        ],
        "Required": []
    }
}
"""
import subprocess
import os

def list_file(file_name: str = "") -> str:
    # Hardcode the directory to /llm_directory for safety
    directory = "llm_directory"
    
    # Validate directory exists
    if not os.path.exists(directory):
        raise FileNotFoundError(f"Directory '{directory}' does not exist")
    
    # Build the ls command
    if file_name:
        # If file_name is provided, search for that specific file/pattern
        cmd = ["ls", "-la", f"{directory}/{file_name}"]
    else:
        # List all files in the directory
        cmd = ["ls", "-la", directory]
    
    # Run the command
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            check=True
        )
        return result.stdout
    except subprocess.CalledProcessError as e:
        return f"Error: {e.stderr}"
    
if __name__ == "__main__":
    # Example usage
    import sys
    
    # Now only accepts optional file_name parameter
    if len(sys.argv) > 1:
        file_name = sys.argv[1]
    else:
        file_name = ""
    
    try:
        result = list_file(file_name)
        print(result)
    except FileNotFoundError as e:
        print(f"Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")
