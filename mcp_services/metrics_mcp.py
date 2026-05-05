import pandas as pd
import pyarrow as pa
import pyarrow.parquet as pq
from fastmcp import FastMCP

mcp = FastMCP("Metrics MCP")
DATA_PATH = "db/data.parquet"

def load_data() -> pd.DataFrame:
    return pd.read_parquet(DATA_PATH)

@mcp.tool()
def get_total_posts() -> int:
    """Get total number of posts/comments in the dataset."""
    df = load_data()
    return int(len(df))

@mcp.tool()
def get_total_engagement() -> int:
    """Get total engagement (likes) across all posts."""
    df = load_data()
    return int(df["liked"].sum()) if "liked" in df.columns else 0

@mcp.tool()
def get_avg_engagement_rate() -> float:
    """Get average engagement rate."""
    df = load_data()
    return float(df["engagementRate"].mean()) if "engagementRate" in df.columns else 0.0

@mcp.tool()
def get_posts_by_sentiment(sentiment: str = "positive") -> int:
    """Get number of posts by sentiment (positive, negative, neutral)."""
    df = load_data()
    return int(len(df[df["sentiment"] == sentiment])) if "sentiment" in df.columns else 0

@mcp.tool()
def get_posts_by_source(source: str) -> int:
    """Get number of posts by source (sourceName)."""
    df = load_data()
    return int(len(df[df["sourceName"] == source])) if "sourceName" in df.columns else 0

@mcp.tool()
def get_top_sources(limit: int = 10) -> dict:
    """Get top sources by post count."""
    df = load_data()
    if "sourceName" not in df.columns:
        return {}
    top = df["sourceName"].value_counts().head(limit)
    return {str(k): int(v) for k, v in top.items()}

@mcp.tool()
def get_engagement_by_source() -> dict:
    """Get engagement (likes) aggregated by source."""
    df = load_data()
    if "liked" not in df.columns or "sourceName" not in df.columns:
        return {}
    eng = df.groupby("sourceName")["liked"].sum().sort_values(ascending=False).head(10)
    return {str(k): int(v) for k, v in eng.items()}

if __name__ == "__main__":
    mcp.run(transport="stdio")