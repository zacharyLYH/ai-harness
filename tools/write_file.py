"""
{
    "Name": "write_file",
    "Description": "Writes content to a file in the llm_directory",
    "NeedUserConsent": true,
    "Params": {
        "Properties": [
            {
                "file_name": {
                    "type": "string",
                    "description": "The name of the file to write"
                }
            },
            {
                "content": {
                    "type": "string",
                    "description": "The content to write to the file"
                }
            }
        ],
        "Required": ["file_name", "content"]
    }
}
"""
import os

def write_file(file_name: str, content: str) -> str:
    """
    Write content to a file in the llm_directory.
    
    Args:
        file_name: The name of the file to write
        content: The content to write to the file
        
    Returns:
        Success message
        
    Raises:
        PermissionError: If the file cannot be written
        OSError: If there's an OS-level error
    """
    # Use relative path to llm_directory in the project root
    directory = "llm_directory"
    file_path = os.path.join(directory, file_name)
    
    # Get absolute paths for validation
    abs_directory = os.path.abspath(directory)
    abs_file_path = os.path.abspath(file_path)
    
    # Validate the file path is within the allowed directory
    if not abs_file_path.startswith(abs_directory):
        raise PermissionError(f"Access denied: File must be within {directory}")
    
    # Ensure directory exists
    os.makedirs(directory, exist_ok=True)
    
    # Write the file
    try:
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
        return f"Successfully wrote to {file_name}"
    except OSError as e:
        raise OSError(f"Failed to write file '{file_name}': {e}")


if __name__ == "__main__":
    # Example usage
    import sys
    
    if len(sys.argv) < 3:
        print("Usage: python write_file.py <file_name> <content>")
        print("Example: python write_file.py example.txt 'Hello, World!'")
        sys.exit(1)
    
    file_name = sys.argv[1]
    content = sys.argv[2]
    
    try:
        result = write_file(file_name, content)
        print(result)
    except PermissionError as e:
        print(f"Error: {e}")
    except OSError as e:
        print(f"Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")