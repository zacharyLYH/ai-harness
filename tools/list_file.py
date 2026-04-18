import subprocess
import os

"""
Lists files in a specified directory using the ls command.

Args:
    file_name: The name of the file to look for (can be pattern or wildcard)
    directory: The directory path to search
    
Returns:
    String containing the output of the ls command
    
Raises:
    FileNotFoundError: If the directory doesn't exist
    subprocess.CalledProcessError: If the ls command fails
"""
def list_file(file_name: str, directory: str) -> str:
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
    
    if len(sys.argv) < 2:
        print("Usage: python list_file.py <directory> [file_name]")
        print("Example: python list_file.py /Users/zac/Desktop/ai-harness")
        print("Example: python list_file.py /Users/zac/Desktop/ai-harness *.go")
        sys.exit(1)
    
    directory = sys.argv[1]
    file_name = sys.argv[2] if len(sys.argv) > 2 else ""
    
    try:
        result = list_file(file_name, directory)
        print(result)
    except FileNotFoundError as e:
        print(f"Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")
