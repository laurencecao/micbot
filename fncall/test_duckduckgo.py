import ollama
from ddgs import DDGS
import json
from datetime import datetime, timedelta
import pytz  # pip install pytz

region="us-en"
region='cn-zh'

# 1. Define the Function
# FunctionGemma relies on the type hints and docstrings to understand the tool.
def search_duckduckgo(query: str) -> str:
    """
    Perform a web search using DuckDuckGo to find information.
    
    Args:
        query: The search query string.
    """
    print(f"\n> Executing Search[{region}] for: '{query}'")
    with DDGS() as ddgs:
        # Get top 3 results
        results = list(ddgs.text(query, region=region, max_results=3))
    return json.dumps(results)

def parse_search_results(results_json: str) -> str:
    """
    Parse DuckDuckGo search results from JSON to readable text.
    
    Args:
        results_json: JSON string containing search results
    """
    try:
        results = json.loads(results_json)
        parsed = []
        for i, result in enumerate(results[:3], 1):
            title = result.get('title', 'No title')
            body = result.get('body', 'No description')
            parsed.append(f"{i}. {title}\n   {body[:150]}...")
        return "\n\n".join(parsed) if parsed else "No results found"
    except Exception as e:
        print(e)
    return results_json

def summarize_news(results_text: str, question: str) -> str:
    """
    Generate a concise answer based on search results.
    This should use a larger LLM, not FunctionGemma.
    
    Args:
        results_text: Parsed search results
        question: Original user question
    """
    # This would typically call a larger model like gemma2:9b or llama3
    summary_prompt = f"""Based on these search results, answer: {question}

Search Results:
{results_text}

Please provide a concise summary of yesterday's main international news headlines."""
    
    # For demonstration - in reality, call a larger model
    response = ollama.chat(
        model='qwen3:4b',  # Or other capable model
        messages=[{'role': 'user', 'content': summary_prompt}]
    )
    return response.message.content

def get_date_info(timezone: str = "Asia/Shanghai") -> dict:
    """
    Get current date, time, and timezone information.
    Use this tool when the user asks about current time, today's date, or time-related queries.
    
    Args:
        timezone: IANA timezone string (default: Asia/Shanghai)
    
    Returns:
        Dictionary with current date, time, and timezone information
    """
    try:
        tz = pytz.timezone(timezone)
        now = datetime.now(tz)
        yesterday = now - timedelta(days=1)
        
        return {
            "current_date": now.strftime("%Y-%m-%d"),
            "current_time": now.strftime("%H:%M:%S"),
            "current_datetime": now.strftime("%Y-%m-%d %H:%M:%S"),
            "yesterday_date": yesterday.strftime("%Y-%m-%d"),
            "timezone": timezone,
            "formatted_time": now.strftime("%Y年%m月%d日 %H时%M分%S秒"),  # Chinese format
            "formatted_yesterday": yesterday.strftime("%Y年%m月%d日"),
        }
    except:
        # Fallback to UTC
        now = datetime.utcnow()
        yesterday = now - timedelta(days=1)
        return {
            "current_date": now.strftime("%Y-%m-%d"),
            "current_time": now.strftime("%H:%M:%S"),
            "current_datetime": now.strftime("%Y-%m-%d %H:%M:%S"),
            "yesterday_date": yesterday.strftime("%Y-%m-%d"),
            "timezone": "UTC",
            "formatted_time": now.strftime("%Y年%m月%d日 %H时%M分%S秒"),
            "formatted_yesterday": yesterday.strftime("%Y年%m月%d日"),
        }
        
# 2. Set up the client and available tools
# We pass the actual function object to Ollama; it handles the schema conversion.
my_tools = [search_duckduckgo, get_date_info]

model_name = "functiongemma:270m"
user_prompt = "昨天的国际新闻头条？"
user_prompt = "现在什么时间？"

print(f"Asking {model_name}: \"{user_prompt}\"...")

# 3. Call the Model
response = ollama.chat(
    model=model_name,
    messages=[{'role': 'user', 'content': user_prompt}],
    tools=my_tools,
)

# 4. Process the Output
# FunctionGemma is "text-in, tool-call-out". It typically returns a tool call, not a chat reply.
if response.message.tool_calls:
    for tool in response.message.tool_calls:
        # Check which function the model wants to call
        if tool.function.name == 'search_duckduckgo':
            # Extract arguments provided by the model
            args = tool.function.arguments
            
            # Execute the function
            search_results = search_duckduckgo(args['query'])
            
            print(f"\n> Raw Result from DuckDuckGo:\n{search_results}")
            
            # Note: Typically, you would feed this result back into a LARGER model 
            # (like Gemma 2 9B or Llama 3) to summarize the answer for the user.
            # FunctionGemma is usually just the "router" or "caller".
           
            # Add these steps:
            parsed_results = parse_search_results(search_results)
            print(f"\n> Parsed Results:\n{parsed_results}")
            
            # Get final answer using a larger model
            final_answer = summarize_news(parsed_results, user_prompt)
            print(f"\n> Answer: {final_answer}")

        if tool.function.name == 'get_date_info':
            # Get yesterday's date
            args = tool.function.arguments if hasattr(tool.function, 'arguments') else {}
            timezone = args.get('timezone', 'Asia/Shanghai')
            
            date_info = get_date_info(timezone)
            
            # Format a user-friendly response
            current_time = date_info.get('formatted_time', date_info.get('current_datetime'))
            timezone_str = date_info.get('timezone', '')
            
            print(f"\n> Current Time Information:")
            print(f"Time: {current_time}")
            print(f"Timezone: {timezone_str}")
            print(f"\n> Full Date Info: {date_info}")
else:
    print("\nThe model did not decide to call a tool. Response:")
    print(response.message.content)