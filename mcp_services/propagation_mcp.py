import pandas as pd
import pyarrow as pa
import pyarrow.parquet as pq
from collections import defaultdict
from fastmcp import FastMCP

mcp = FastMCP("Propagation MCP")
DATA_PATH = "db/data.parquet"

def load_data() -> pd.DataFrame:
    return pd.read_parquet(DATA_PATH)

def build_reply_tree(df: pd.DataFrame) -> dict:
    """Build reply tree: parent_id -> [child_ids]"""
    tree = defaultdict(list)
    if "parentId" in df.columns and "id" in df.columns:
        for _, row in df.iterrows():
            parent = row.get("parentId")
            child = row.get("id")
            if pd.notna(parent) and pd.notna(child):
                tree[str(parent)].append(str(child))
    return dict(tree)

def calculate_reach(df: pd.DataFrame, post_id: str) -> int:
    """Calculate accumulated reach for a post (includes all replies)."""
    tree = build_reply_tree(df)
    visited = set()
    queue = [post_id]
    total = 0
    while queue:
        current = queue.pop(0)
        if current in visited:
            continue
        visited.add(current)
        total += 1
        children = tree.get(current, [])
        queue.extend(children)
    return total

@mcp.tool()
def get_post_depth(post_id: str) -> int:
    """Get reply depth for a specific post."""
    df = load_data()
    tree = build_reply_tree(df)
    depth = 0
    current = post_id
    visited = set()
    while current in tree and current not in visited:
        visited.add(current)
        children = tree.get(current, [])
        if children:
            current = children[0]
            depth += 1
        else:
            break
    return depth

@mcp.tool()
def get_reach(post_id: str) -> int:
    """Get total reach (original post + all replies)."""
    df = load_data()
    return calculate_reach(df, post_id)

@mcp.tool()
def get_top_posts_by_reach(limit: int = 10) -> dict:
    """Get posts with highest reach."""
    df = load_data()
    reach_data = {}
    for _, row in df.iterrows():
        post_id = row.get("id")
        if pd.notna(post_id):
            reach_data[str(post_id)] = calculate_reach(df, str(post_id))
    sorted_posts = sorted(reach_data.items(), key=lambda x: x[1], reverse=True)[:limit]
    return {k: v for k, v in sorted_posts}

@mcp.tool()
def get_thread_sizes() -> dict:
    """Get size of each thread (number of replies per post)."""
    df = load_data()
    tree = build_reply_tree(df)
    thread_sizes = {parent: len(children) for parent, children in tree.items()}
    sorted_threads = sorted(thread_sizes.items(), key=lambda x: x[1], reverse=True)[:10]
    return {str(k): int(v) for k, v in sorted_threads}

@mcp.tool()
def get_original_posts() -> dict:
    """Get all original posts (without parent)."""
    df = load_data()
    if "parentId" not in df.columns:
        return {}
    originals = df[df["parentId"].isna() | (df["parentId"] == "")]
    return {"count": int(len(originals)), "sample_ids": [str(x) for x in originals["id"].head(5).tolist()]}

if __name__ == "__main__":
    mcp.run(transport="stdio")