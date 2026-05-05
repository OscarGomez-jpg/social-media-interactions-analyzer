import os
import re
import pandas as pd
import pyarrow as pa
import pyarrow.parquet as pq
from fastmcp import FastMCP

mcp = FastMCP("Summary MCP")
DATA_PATH = "db/data.parquet"

def load_data() -> pd.DataFrame:
    return pd.read_parquet(DATA_PATH)

def clean_text(text: str) -> str:
    if pd.isna(text):
        return ""
    text = str(text)
    text = re.sub(r'http\S+|www\.\S+', '', text)
    text = re.sub(r'[\U0001F600-\U0001F64F\U0001F300-\U0001F5FF\U0001F680-\U0001F6FF\U0001F1E0-\U0001F1FF]', '', text)
    text = re.sub(r'@\w+', '', text)
    text = re.sub(r'#\w+', '', text)
    text = re.sub(r'\s+', ' ', text)
    return text.strip()

def get_client():
    from openai import OpenAI
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        return None
    return OpenAI(api_key=api_key)

@mcp.tool()
def get_sample_posts(limit: int = 20) -> dict:
    """Get sample cleaned posts for review."""
    df = load_data()
    posts = []
    for _, row in df.head(limit).iterrows():
        text = row.get("text", "")
        parent_text = row.get("parentText", "")
        combined = f"{text} {parent_text}" if pd.notna(parent_text) else str(text) if pd.notna(text) else ""
        cleaned = clean_text(combined)
        if cleaned:
            posts.append({
                "id": str(row.get("id", "")),
                "cleaned_text": cleaned[:500]
            })
    return {"posts": posts}

@mcp.tool()
def get_posts_by_keyword(keyword: str, limit: int = 50) -> list:
    """Get posts containing a keyword."""
    df = load_data()
    if "text" not in df.columns:
        return []
    mask = df["text"].astype(str).str.lower().str.contains(keyword.lower(), na=False)
    filtered = df[mask].head(limit)
    return [clean_text(str(row.get("text", ""))) for _, row in filtered.iterrows() if pd.notna(row.get("text"))]

@mcp.tool()
def summarize_posts_by_sentiment(sentiment: str = "positive") -> dict:
    """Summarize thematic topics from posts by sentiment using LLM.

    Returns a structured dict so callers can always distinguish a real
    summary from an error condition:
      {"available": True,  "summary": "..."}
      {"available": False, "error": "reason"}
    """
    df = load_data()
    if "sentiment" not in df.columns:
        return {"available": False, "error": "No sentiment column in dataset"}

    client = get_client()
    if not client:
        return {"available": False, "error": "OPENAI_API_KEY is not configured"}

    filtered = df[df["sentiment"] == sentiment].head(30)
    texts = [
        clean_text(str(row.get("text", "")))
        for _, row in filtered.iterrows()
        if pd.notna(row.get("text"))
    ]

    if not texts:
        return {"available": False, "error": f"No posts found with sentiment: {sentiment}"}

    combined = "\n\n".join([f"- {t[:300]}" for t in texts[:15]])

    prompt = f"""Analyze these social media posts and identify the main themes and topics being discussed.
Provide a brief summary (2-3 sentences) of what people are saying:

{combined}

Summary:"""

    try:
        response = client.chat.completions.create(
            model="gpt-4o-mini",
            messages=[{"role": "user", "content": prompt}],
            max_tokens=200,
        )
        return {"available": True, "summary": response.choices[0].message.content}
    except Exception as e:
        return {"available": False, "error": f"LLM call failed: {str(e)}"}

@mcp.tool()
def get_top_keywords(limit: int = 20) -> dict:
    """Get most common keywords/tags from posts."""
    df = load_data()
    keywords_all = []
    if "keywords" in df.columns:
        for kw in df["keywords"].dropna():
            if isinstance(kw, list):
                keywords_all.extend(kw)
            elif isinstance(kw, str):
                keywords_all.extend([k.strip() for k in kw.split(",")])
    
    if "tags" in df.columns:
        for tag in df["tags"].dropna():
            if isinstance(tag, list):
                keywords_all.extend(tag)
            elif isinstance(tag, str):
                keywords_all.extend([t.strip() for t in tag.split(",")])
    
    from collections import Counter
    counts = Counter(keywords_all)
    top = counts.most_common(limit)
    return {str(k): int(v) for k, v in top}

if __name__ == "__main__":
    mcp.run(transport="stdio")