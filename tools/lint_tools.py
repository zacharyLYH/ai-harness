#!/usr/bin/env python3
"""
Linting script for AI Harness tools.
Checks Python files in the workspace for compliance with project standards.
"""

import os
import re
import json
import sys
from pathlib import Path
from typing import List, Dict, Any, Tuple, Optional

# ANSI color codes for terminal output
class Colors:
    RED = '\033[91m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    MAGENTA = '\033[95m'
    CYAN = '\033[96m'
    WHITE = '\033[97m'
    RESET = '\033[0m'
    BOLD = '\033[1m'

def print_header(message: str) -> None:
    """Print a header message."""
    print(f"\n{Colors.CYAN}{'=' * 80}{Colors.RESET}")
    print(f"{Colors.BOLD}{Colors.CYAN}{message}{Colors.RESET}")
    print(f"{Colors.CYAN}{'=' * 80}{Colors.RESET}")

def print_success(message: str) -> None:
    """Print a success message."""
    print(f"{Colors.GREEN}✓ {message}{Colors.RESET}")

def print_warning(message: str) -> None:
    """Print a warning message."""
    print(f"{Colors.YELLOW}⚠ {message}{Colors.RESET}")

def print_error(message: str) -> None:
    """Print an error message."""
    print(f"{Colors.RED}✗ {message}{Colors.RESET}")

def find_python_files(workspace_root: str) -> List[str]:
    """
    Find all Python files in the workspace.
    
    Args:
        workspace_root: Root directory of the workspace
        
    Returns:
        List of absolute paths to Python files
    """
    python_files = []
    
    for root, dirs, files in os.walk(workspace_root):
        # Skip hidden directories like .git
        dirs[:] = [d for d in dirs if not d.startswith('.')]
        
        for file in files:
            if file.endswith('.py'):
                filepath = os.path.join(root, file)
                # Exclude the linter itself
                if file != "lint_tools.py":
                    python_files.append(filepath)
    
    return sorted(python_files)

def parse_metadata(content: str) -> Tuple[Optional[Dict[str, Any]], int]:
    """
    Parse JSON metadata from a Python file.
    
    Args:
        content: Content of the Python file
        
    Returns:
        Tuple of (metadata_dict, end_line_of_metadata)
    """
    lines = content.split('\n')
    
    # Look for the opening triple-quote comment
    metadata_start = -1
    for i, line in enumerate(lines):
        if '"""' in line and '{' in content:
            metadata_start = i
            break
    
    if metadata_start == -1:
        return None, -1
    
    # Find the closing triple-quote
    metadata_lines = []
    for i in range(metadata_start, len(lines)):
        line = lines[i]
        metadata_lines.append(line)
        if i > metadata_start and '"""' in line:
            # Join lines and try to parse JSON
            metadata_text = '\n'.join(metadata_lines)
            
            # Extract just the JSON content between triple quotes
            match = re.search(r'"""\s*(.*?)\s*"""', metadata_text, re.DOTALL)
            if match:
                json_text = match.group(1)
                try:
                    metadata = json.loads(json_text)
                    return metadata, i  # Return metadata and end line
                except json.JSONDecodeError:
                    return None, -1
    
    return None, -1

def extract_function_name(content: str, filename: str) -> Optional[str]:
    """
    Extract function name from Python file content.
    Tries to find a function matching the filename, otherwise returns first function.
    
    Args:
        content: Content of the Python file
        filename: Name of the file without .py extension
        
    Returns:
        Function name or None if no function found
    """
    # Look for all function definition patterns
    pattern = r'def\s+(\w+)\s*\('
    all_functions = re.findall(pattern, content)
    
    if not all_functions:
        return None
    
    # First try to find function matching filename
    for func_name in all_functions:
        if func_name == filename:
            return func_name
    
    # If no match, return the first function
    return all_functions[0]

def check_file_structure(filepath: str) -> Dict[str, Any]:
    """
    Check a Python file for compliance with project standards.
    
    Args:
        filepath: Path to the Python file
        
    Returns:
        Dictionary with linting results
    """
    results = {
        'filepath': filepath,
        'filename': os.path.basename(filepath),
        'has_metadata': False,
        'metadata_valid': False,
        'filename_matches_function': False,
        'has_directory_constraint': False,
        'function_name': None,
        'expected_filename': None,
        'metadata': None,
        'errors': [],
        'warnings': [],
        'successes': []
    }
    
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            content = f.read()
    except Exception as e:
        results['errors'].append(f"Could not read file: {e}")
        return results
    
    # Check 1: Extract function name
    filename_without_ext = os.path.splitext(results['filename'])[0]
    function_name = extract_function_name(content, filename_without_ext)
    if function_name:
        results['function_name'] = function_name
        results['expected_filename'] = f"{function_name}.py"
        
        # Check if filename matches function name
        if results['filename'] == results['expected_filename']:
            results['filename_matches_function'] = True
            results['successes'].append(f"Filename matches function name: {function_name}")
        else:
            # Only error if function doesn't match filename AND isn't the first function
            if function_name != filename_without_ext:
                results['errors'].append(
                    f"Filename '{results['filename']}' does not match function name '{function_name}'. "
                    f"Expected: {function_name}.py"
                )
    else:
        results['errors'].append("No function definition found in file")
    
    # Check 2: Parse metadata
    metadata, metadata_end_line = parse_metadata(content)
    
    if metadata:
        results['has_metadata'] = True
        results['metadata'] = metadata
        
        # Validate metadata structure
        required_fields = ['Name', 'Description', 'NeedUserConsent', 'Params']
        
        for field in required_fields:
            if field not in metadata:
                results['errors'].append(f"Metadata missing required field: {field}")
        
        # Validate NeedUserConsent is a boolean
        if 'NeedUserConsent' in metadata:
            if not isinstance(metadata['NeedUserConsent'], bool):
                results['errors'].append(
                    f"Metadata field 'NeedUserConsent' must be a boolean (true/false), "
                    f"got {type(metadata['NeedUserConsent']).__name__}"
                )
            else:
                results['successes'].append(f"NeedUserConsent is set to {metadata['NeedUserConsent']}")
        
        if not results['errors']:
            results['metadata_valid'] = True
            results['successes'].append("Metadata is valid and complete")
        
        # Check if Name in metadata matches function name
        if function_name and 'Name' in metadata:
            if metadata['Name'] == function_name:
                results['successes'].append("Metadata 'Name' field matches function name")
            else:
                results['errors'].append(
                    f"Metadata 'Name' field '{metadata['Name']}' does not match "
                    f"function name '{function_name}'"
                )
    else:
        results['errors'].append("No valid metadata found. Expected JSON metadata in triple-quoted comments at top of file")
    
    # Check 3: Look for directory constraint
    llm_dir_patterns = [
        r'/llm_directory',
        r'llm_directory',
        r'os\.chdir.*llm',
        r'os\.path\.join.*llm'
    ]
    
    for pattern in llm_dir_patterns:
        if re.search(pattern, content, re.IGNORECASE):
            results['has_directory_constraint'] = True
            break
    
    if results['has_directory_constraint']:
        results['successes'].append("File appears to constrain operations to /llm_directory")
    else:
        results['warnings'].append("File may not constrain operations to /llm_directory")
    
    # Check 4: Check for docstring format - look for triple quotes anywhere in first 10 lines
    lines = content.split('\n')
    has_triple_quotes = False
    for i, line in enumerate(lines[:10]):
        if '"""' in line:
            has_triple_quotes = True
            break
    
    if not has_triple_quotes:
        results['warnings'].append("File does not have metadata/docstring in first 10 lines")
    
    return results

def print_results(results: Dict[str, Any]) -> None:
    """
    Print linting results for a file.
    
    Args:
        results: Linting results dictionary
    """
    filename = os.path.basename(results['filepath'])
    print(f"\n{Colors.BOLD}{filename}{Colors.RESET} ({results['filepath']})")
    
    if results['successes']:
        for success in results['successes']:
            print_success(success)
    
    if results['warnings']:
        for warning in results['warnings']:
            print_warning(warning)
    
    if results['errors']:
        for error in results['errors']:
            print_error(error)
    
    # Print summary
    if results['errors']:
        print(f"{Colors.RED}✗ FAIL{Colors.RESET}")
    elif results['warnings']:
        print(f"{Colors.YELLOW}⚠ WARNINGS{Colors.RESET}")
    else:
        print(f"{Colors.GREEN}✓ PASS{Colors.RESET}")

def generate_report(all_results: List[Dict[str, Any]]) -> Dict[str, Any]:
    """
    Generate a summary report of all linting results.
    
    Args:
        all_results: List of all file results
        
    Returns:
        Summary statistics
    """
    total_files = len(all_results)
    passed_files = 0
    warning_files = 0
    failed_files = 0
    
    for results in all_results:
        if results['errors']:
            failed_files += 1
        elif results['warnings']:
            warning_files += 1
        else:
            passed_files += 1
    
    return {
        'total_files': total_files,
        'passed_files': passed_files,
        'warning_files': warning_files,
        'failed_files': failed_files
    }

def print_summary(report: Dict[str, Any]) -> None:
    """
    Print a summary of the linting results.
    
    Args:
        report: Summary statistics
    """
    print_header("LINTING SUMMARY")
    
    print(f"Total files checked: {report['total_files']}")
    print(f"{Colors.GREEN}Passed: {report['passed_files']}{Colors.RESET}")
    print(f"{Colors.YELLOW}Warnings: {report['warning_files']}{Colors.RESET}")
    print(f"{Colors.RED}Failed: {report['failed_files']}{Colors.RESET}")
    
    if report['failed_files'] > 0:
        print(f"\n{Colors.RED}Some files failed linting checks.{Colors.RESET}")
        sys.exit(1)
    elif report['warning_files'] > 0:
        print(f"\n{Colors.YELLOW}All files passed, but some have warnings.{Colors.RESET}")
    else:
        print(f"\n{Colors.GREEN}All files passed linting checks!{Colors.RESET}")

def main() -> None:
    """Main function for the linting script."""
    # Get workspace root (parent directory of this script)
    workspace_root = os.path.dirname(os.path.abspath(__file__))
    
    # Find all Python files
    python_files = find_python_files(workspace_root)
    
    if not python_files:
        return
    
    # Check each file
    all_results = []
    for filepath in python_files:
        results = check_file_structure(filepath)
        all_results.append(results)
    
    # Generate report
    report = generate_report(all_results)
    
    # Only print output if there are failures
    if report['failed_files'] > 0:
        print_header("AI HARNESS TOOLS LINTER")
        print(f"Workspace: {workspace_root}")
        print(f"Found {len(python_files)} Python file(s)")
        
        for results in all_results:
            if results['errors'] or results['warnings']:
                print_results(results)
        
        print_summary(report)

if __name__ == "__main__":
    main()