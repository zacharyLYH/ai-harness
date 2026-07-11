"""
{
    "Name": "curl_web",
    "Description": "Fetches a webpage via curl and returns the core text content with HTML stripped out",
    "Params": {
        "Properties": [
            {
                "url": {
                    "type": "string",
                    "description": "The URL to fetch"
                }
            }
        ],
        "Required": ["url"]
    }
}
"""
import re
import subprocess
import sys
import html as html_module


def strip_html(html_text: str, max_chars: int = 5000) -> str:
    """
    Aggressively strip HTML to return only the core readable text content.

    Steps:
    1. Remove HTML comments
    2. Remove script, style, noscript, link, meta, svg, canvas blocks entirely
    3. Replace block-level tags with newlines
    4. Remove all remaining HTML tags
    5. Decode HTML entities
    6. Collapse excessive whitespace
    7. Remove empty/whitespace-only lines
    8. Trim to max_chars
    """
    if not html_text:
        return ""

    text = html_text

    # 1. Remove HTML comments
    text = re.sub(r'<!--.*?-->', '', text, flags=re.DOTALL)

    # 2. Remove entire blocks of non-content tags
    blocks_to_remove = [
        r'<script[^>]*>.*?</script>',
        r'<style[^>]*>.*?</style>',
        r'<noscript[^>]*>.*?</noscript>',
        r'<svg[^>]*>.*?</svg>',
        r'<canvas[^>]*>.*?</canvas>',
    ]
    for pattern in blocks_to_remove:
        text = re.sub(pattern, '', text, flags=re.DOTALL | re.IGNORECASE)

    # 3. Replace block-level tags with newlines to separate text
    block_tags = [
        r'<\s*br\s*/?\s*>',
        r'<\s*/?\s*(p|div|h[1-6]|li|tr|td|th|blockquote|pre|section|article|header|footer|nav|aside|dd|dt|dl|ol|ul|hr|figure|figcaption|details|summary|main)\s*>',
    ]
    for pattern in block_tags:
        text = re.sub(pattern, '\n', text, flags=re.IGNORECASE)

    # 4. Remove all remaining HTML tags (including attributes)
    text = re.sub(r'<[^>]+>', '', text)

    # 5. Decode HTML entities
    text = html_module.unescape(text)

    # 6. Decode common unicode escapes
    text = text.replace('\\n', '\n').replace('\\t', ' ')

    # 7. Collapse whitespace: replace multiple spaces/tabs with single space
    text = re.sub(r'[ \t]+', ' ', text)
    text = re.sub(r'\n{3,}', '\n\n', text)

    # 8. Remove leading/trailing whitespace on each line and drop empty lines
    lines = text.split('\n')
    cleaned_lines = []
    for line in lines:
        stripped = line.strip()
        if stripped:
            cleaned_lines.append(stripped)

    text = '\n'.join(cleaned_lines)

    # 9. Trim to max_chars (cut at word boundary if possible)
    if len(text) > max_chars:
        text = text[:max_chars]
        last_space = text.rfind(' ')
        if last_space > max_chars * 0.85:
            text = text[:last_space]
        text += '\n... [truncated]'

    return text


def curl_web(url: str) -> str:
    """
    Fetch a webpage and return the core text content.

    Args:
        url: The URL to fetch

    Returns:
        String containing the stripped text content of the page
    """
    # Validate URL starts with http
    if not url.startswith(('http://', 'https://')):
        return "Error: URL must start with http:// or https://"

    try:
        result = subprocess.run(
            ['curl', '-s', '-L', '--max-time', '15', url],
            capture_output=True,
            text=True,
            timeout=20
        )

        if result.returncode != 0:
            error_msg = result.stderr.strip() if result.stderr else f"curl failed with exit code {result.returncode}"
            return f"Error fetching URL: {error_msg}"

        html_content = result.stdout
        if not html_content:
            return "Error: No content returned from URL"

        stripped = strip_html(html_content)
        return stripped if stripped else "No readable text content found on the page"

    except subprocess.TimeoutExpired:
        return "Error: Request timed out after 20 seconds"
    except FileNotFoundError:
        return "Error: curl command not found. Please install curl."
    except Exception as e:
        return f"Error: {str(e)}"


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python curl_web.py <url>")
        print("Example: python curl_web.py https://example.com")
        sys.exit(1)

    url = sys.argv[1]
    result = curl_web(url)
    print(result)