"""
{
    "Name": "search_for_files",
    "Description": "Searches for a pattern across all files in the llm_directory",
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
import sys

def search_for_files(search_pattern: str) -> str:
    """
    Search for a pattern across all files in the llm_directory.
    
    Args:
        search_pattern: The pattern to search for in all files
        
    Returns:
        String containing matching files and lines, or a message if no matches found
        
    Raises:
        FileNotFoundError: If the llm_directory doesn't exist
        PermissionError: If the directory cannot be accessed
    """
    # Hardcode the directory to /llm_directory for safety
    directory = "llm_directory"
    
    # Validate the directory exists
    if not os.path.exists(directory):
        raise FileNotFoundError(f"Directory '{directory}' does not exist")
    
    # Check if it's a directory
    if not os.path.isdir(directory):
        raise ValueError(f"'{directory}' is not a directory")
    
    # Get all files in the directory
    all_files = []
    for root, dirs, files in os.walk(directory):
        for file in files:
            file_path = os.path.join(root, file)
            all_files.append(file_path)
    
    if not all_files:
        return f"No files found in {directory}"
    
    # Search for pattern in each file
    results = []
    total_matches = 0
    
    for file_path in all_files:
        try:
            # Try to read the file
            with open(file_path, 'r', encoding='utf-8') as f:
                content = f.read()
        except UnicodeDecodeError:
            # Skip binary files or try with errors='ignore'
            try:
                with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
                    content = f.read()
            except:
                continue  # Skip this file if we can't read it
        except:
            continue  # Skip files we can't read
            
        # Split content into lines and search for pattern
        lines = content.split('\n')
        file_matches = []
        
        for line_num, line in enumerate(lines, 1):
            if search_pattern in line:
                file_matches.append(f"  Line {line_num}: {line.strip()}")
                total_matches += 1
        
        # If we found matches in this file, add to results
        if file_matches:
            # Get relative path from the llm_directory
            rel_path = os.path.relpath(file_path, directory)
            results.append(f"File: {rel_path}")
            results.extend(file_matches)
            results.append("")  # Empty line between files
    
    # Format the results
    if results:
        header = f"Found {total_matches} match{'es' if total_matches != 1 else ''} for pattern '{search_pattern}' in {len([r for r in results if r.startswith('File:')])} file{'s' if len([r for r in results if r.startswith('File:')]) != 1 else ''}:\n"
        return header + "\n".join(results).strip()
    else:
        return f"No matches found for pattern '{search_pattern}' in any files"


if __name__ == "__main__":
    # Example usage
    if len(sys.argv) < 2:
        print("Usage: python search_for_files.py <search_pattern>")
        print("Example: python search_for_files.py 'hello'")
        sys.exit(1)
    
    search_pattern = sys.argv[1]
    
    try:
        result = search_for_files(search_pattern)
        print(result)
    except FileNotFoundError as e:
        print(f"Error: {e}")
    except PermissionError as e:
        print(f"Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")
