"""
{
    "Name": "grepper",
    "Description": "Searches for a pattern across all files in the llm_directory using grep",
    "NeedUserConsent": false,
    "Params": {
        "Properties": [
            {
                "search_pattern": {
                    "type": "string",
                    "description": "The pattern to search for across all files"
                }
            }
        ],
        "Required": ["search_pattern"]
    }
}
"""
import os
import subprocess
import sys

def grepper(search_pattern: str) -> str:
    """
    Search for a pattern across all files in the llm_directory using the grep command.
    
    Args:
        search_pattern: The pattern to search for
        
    Returns:
        String containing grep output with matching files and lines
        
    Raises:
        FileNotFoundError: If the llm_directory doesn't exist
    """
    directory = "llm_directory"
    
    if not os.path.exists(directory):
        raise FileNotFoundError(f"Directory '{directory}' does not exist")
    
    cmd = ["grep", "-rn", search_pattern, directory]
    
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
        output = result.stdout
        if not output:
            return f"No matches found for pattern '{search_pattern}' in {directory}"
        return output.strip()
    except subprocess.CalledProcessError as e:
        # grep returns exit code 1 when no matches found
        if e.returncode == 1:
            return f"No matches found for pattern '{search_pattern}' in {directory}"
        return f"Error: {e.stderr}"


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python grepper.py <search_pattern>")
        print("Example: python grepper.py 'hello'")
        sys.exit(1)
    
    search_pattern = sys.argv[1]
    
    try:
        result = grepper(search_pattern)
        print(result)
    except FileNotFoundError as e:
        print(f"Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")