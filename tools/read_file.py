"""
{
    "Name": "read_file",
    "Description": "Reads the contents of a file from the llm_directory",
    "Params": {
        "Properties": [
            {
                "file_name": {
                    "type": "string",
                    "description": "The name of the file to read"
                }
            }
        ],
        "Required": ["file_name"]
    }
}
"""
import os

def read_file(file_name: str) -> str:
    """
    Read the contents of a file from the llm_directory.
    
    Args:
        file_name: The name of the file to read
        
    Returns:
        String containing the file contents
        
    Raises:
        FileNotFoundError: If the file doesn't exist
        PermissionError: If the file cannot be read
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
    
    # Check if file exists
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"File '{file_name}' does not exist in {directory}")
    
    # Check if it's a file
    if not os.path.isfile(file_path):
        raise ValueError(f"'{file_name}' is not a file")
    
    # Read the file
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            return f.read()
    except UnicodeDecodeError:
        # Try reading as binary if utf-8 fails
        with open(file_path, 'rb') as f:
            return f.read().decode('utf-8', errors='ignore')


if __name__ == "__main__":
    # Example usage
    import sys
    
    if len(sys.argv) < 2:
        print("Usage: python read_file.py <file_name>")
        print("Example: python read_file.py example.txt")
        sys.exit(1)
    
    file_name = sys.argv[1]
    
    try:
        result = read_file(file_name)
        print(result)
    except FileNotFoundError as e:
        print(f"Error: {e}")
    except PermissionError as e:
        print(f"Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")