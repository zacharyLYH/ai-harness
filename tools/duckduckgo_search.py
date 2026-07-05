"""
{
    "Name": "duckduckgo_search",
    "Description": "Searches DuckDuckGo for a query and returns a clean text summary of the results",
    "NeedUserConsent": false,
    "Params": {
        "Properties": [
            {
                "query": {
                    "type": "string",
                    "description": "The search query to send to DuckDuckGo"
                }
            }
        ],
        "Required": ["query"]
    }
}
"""
import json
import subprocess
import sys
import urllib.parse


def duckduckgo_search(query: str) -> str:
    """
    Search DuckDuckGo for the given query and return formatted results.
    Uses DuckDuckGo's Instant Answer API (no CAPTCHA required).
    
    Args:
        query: The search query
        
    Returns:
        String containing the search results
    """
    if not query or not query.strip():
        return "Error: Search query cannot be empty"
    
    # Use DuckDuckGo's Instant Answer API (JSON, no CAPTCHA)
    encoded_query = urllib.parse.quote(query.strip())
    url = f"https://api.duckduckgo.com/?q={encoded_query}&format=json"
    
    try:
        result = subprocess.run(
            ['curl', '-s', '--max-time', '15', 
             '-A', 'Mozilla/5.0 (compatible; SearchBot/1.0)',
             url],
            capture_output=True,
            text=True,
            timeout=20
        )
        
        if result.returncode != 0:
            error_msg = result.stderr.strip() if result.stderr else f"curl failed with exit code {result.returncode}"
            return f"Error searching DuckDuckGo: {error_msg}"
        
        response_text = result.stdout
        if not response_text:
            return "Error: No response from DuckDuckGo API"
        
        # Parse JSON response
        try:
            data = json.loads(response_text)
        except json.JSONDecodeError:
            return "Error: Failed to parse DuckDuckGo API response"
        
        output_parts = []
        
        # Abstract text (featured snippet / knowledge panel)
        abstract = data.get("AbstractText", "")
        if abstract:
            abstract_source = data.get("AbstractSource", "")
            abstract_url = data.get("AbstractURL", "")
            output_parts.append(f"Featured snippet: {abstract}")
            if abstract_source:
                output_parts.append(f"Source: {abstract_source}")
            output_parts.append("")
        
        # Heading / definition
        heading = data.get("Heading", "")
        if heading:
            definition = data.get("Definition", "")
            if definition:
                output_parts.append(f"{heading}: {definition}")
                output_parts.append("")
        
        # Answer (direct answer, e.g. calculations, conversions)
        answer = data.get("Answer", "")
        answer_type = data.get("AnswerType", "")
        if answer:
            output_parts.append(f"Answer: {answer}")
            if answer_type:
                output_parts.append(f"Type: {answer_type}")
            output_parts.append("")
        
        # Results (web results)
        results = data.get("Results", [])
        if results:
            output_parts.append("Results:")
            for i, r in enumerate(results[:8], 1):
                title = r.get("Text", "")
                result_url = r.get("FirstURL", "")
                snippet = ""  
                if title:
                    output_parts.append(f"  {i}. {title}")
                if result_url:
                    output_parts.append(f"     {result_url}")
                if r.get("Snippet"):
                    output_parts.append(f"     {r['Snippet']}")
            output_parts.append("")
        
        # Related topics
        related = data.get("RelatedTopics", [])
        if related:
            output_parts.append("Related:")
            count = 0
            for item in related:
                if count >= 5:
                    break
                if "Text" in item:  # Direct topic
                    text = item.get("Text", "")
                    url = item.get("FirstURL", "")
                    if text:
                        output_parts.append(f"  • {text}")
                        count += 1
                elif "Topics" in item:  # Nested topics
                    for t in item["Topics"]:
                        if count >= 5:
                            break
                        text = t.get("Text", "")
                        if text:
                            output_parts.append(f"  • {text}")
                            count += 1
        
        # If nothing was found
        if not output_parts:
            return f"No results found for '{query}'"
        
        output = f"Search results for '{query}':\n\n"
        output += '\n'.join(output_parts)
        
        return output.strip()
        
    except subprocess.TimeoutExpired:
        return "Error: Request timed out after 20 seconds"
    except FileNotFoundError:
        return "Error: curl command not found. Please install curl."
    except Exception as e:
        return f"Error: {str(e)}"


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python duckduckgo_search.py <query>")
        print("Example: python duckduckgo_search.py 'latest AI news'")
        sys.exit(1)
    
    query = ' '.join(sys.argv[1:])
    result = duckduckgo_search(query)
    print(result)
