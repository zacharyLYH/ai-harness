"""
{
    "Name": "search_in_file",
    "Description": "Searches for a pattern in a file within the llm_directory",
    "Params": {
        "Properties": [
            {
                "file_name": {
                    "type": "string",
                    "description": "The name of the file to search in"
                }
            },
            {
                "search_pattern": {
                    "type": "string",
                    "description": "The pattern to search for in the file"
                }
            }
        ],
        "Required": ["file_name", "search_pattern"]
    }
}
"""
import os
import re

def search_in_file(file_name: str, search_pattern: str) -> str:
    """
    Search for a pattern in a file within the llm_directory.
    
    Args:
        file_name: The name of the file to search in
        search_pattern: The pattern to search for in the file
        
    Returns:
        String containing matching lines or a message if no matches found
        
    Raises:
        FileNotFoundError: If the file doesn't exist
        PermissionError: If the file cannot be read
    """
    # Hardcode the directory to /llm_directory for safety
    directory = "llm_directory"
    file_path = os.path.join(directory, file_name)
    
    # Validate the file path is within the allowed directory
    if not os.path.commonpath([os.path.realpath(file_path), directory]) == directory:
        raise PermissionError(f"Access denied: File must be within {directory}")
    
    # Check if file exists
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"File '{file_name}' does not exist in {directory}")
    
    # Check if it's a file
    if not os.path.isfile(file_path):
        raise ValueError(f"'{file_name}' is not a file")
    
    # Read the file and search for pattern
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            content = f.read()
    except UnicodeDecodeError:
        # Try reading as binary if utf-8 fails
        with open(file_path, 'rb') as f:
            content = f.read().decode('utf-8', errors='ignore')
    
    # Search for pattern (simple string search, could be extended to regex)
    lines = content.split('\n')
    matches = []
    
    for line_num, line in enumerate(lines, 1):
        if search_pattern in line:
            matches.append(f"Line {line_num}: {line}")
    
    if matches:
        return "\n".join(matches)
    else:
        return f"No matches found for pattern '{search_pattern}' in {file_name}"


if __name__ == "__main__":
    # Example usage
    import sys
    
    if len(sys.argv) < 3:
        print("Usage: python search_in_file.py <file_name> <search_pattern>")
        print("Example: python search_in_file.py example.txt 'hello'")
        sys.exit(1)
    
    file_name = sys.argv[1]
    search_pattern = sys.argv[2]
    
    try:
        result = search_in_file(file_name, search_pattern)
        print(result)
    except FileNotFoundError as e:
        print(f"Error: {e}")
    except PermissionError as e:
        print(f"Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")