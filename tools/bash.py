"""
{
    "Name": "bash",
    "Description": "Execute a bash command on the system. Provide the command and a plain english description of what it does.",
    "Params": {
        "Properties": [
            {
                "command": {
                    "type": "string",
                    "description": "The bash command to execute"
                }
            },
            {
                "description": {
                    "type": "string",
                    "description": "A plain english explanation of what this command is used for"
                }
            }
        ],
        "Required": ["command", "description"]
    }
}
"""
import subprocess
import sys


def bash(command: str, description: str) -> str:
    """
    Execute a bash command on the system.
    Does NOT run commands with sudo.

    Args:
        command: The bash command to execute
        description: A plain english explanation of what this command is used for

    Returns:
        String containing the stdout and stderr of the command

    Raises:
        PermissionError: If the command contains sudo
    """
    if command.strip().startswith("sudo"):
        raise PermissionError("sudo is not allowed. Please run the command without sudo.")

    result = subprocess.run(
        command,
        shell=True,
        capture_output=True,
        text=True,
        timeout=30
    )

    output = result.stdout
    if result.stderr:
        output += "\nSTDERR:\n" + result.stderr

    if result.returncode != 0:
        output += f"\nExit code: {result.returncode}"

    return output


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python bash.py <command> <description>")
        sys.exit(1)

    command = sys.argv[1]
    description = sys.argv[2]

    try:
        result = bash(command, description)
        print(result)
    except PermissionError as e:
        print(f"Permission denied: {e}")
    except Exception as e:
        print(f"Error: {e}")