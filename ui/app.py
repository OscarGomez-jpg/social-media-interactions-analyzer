import streamlit as st
import requests
import json
import pandas as pd
import plotly.express as px

BACKEND_URL = "http://localhost:8080"
METRICS_MCP_URL = "http://localhost:8001"

st.set_page_config(page_title="Social Media Analyzer", layout="wide")

# ── Sidebar ────────────────────────────────────────────────────────────────────
st.sidebar.header("Configuration")
backend_url = st.sidebar.text_input("Backend URL", value=BACKEND_URL, key="backend_url_input")
metrics_url = st.sidebar.text_input("Metrics MCP URL", value=METRICS_MCP_URL, key="metrics_url_input")

# ── MCP helper ─────────────────────────────────────────────────────────────────
def call_mcp(base_url: str, method: str, params: dict | None = None) -> dict | None:
    """Call a FastMCP tool via JSON-RPC 2.0 and return the result, or None on error."""
    payload = {"jsonrpc": "2.0", "method": method, "params": params or {}, "id": 1}
    try:
        resp = requests.post(base_url, json=payload, timeout=10)
        resp.raise_for_status()
        data = resp.json()
        if data.get("error"):
            st.error(f"MCP error ({method}): {data['error']['message']}")
            return None
        return data.get("result")
    except requests.exceptions.ConnectionError:
        st.warning(f"⚠️ Cannot connect to MCP at {base_url} — is the service running?")
        return None
    except Exception as exc:
        st.error(f"Unexpected error calling {method}: {exc}")
        return None

@st.cache_data(ttl=60)
def fetch_metrics(base_url: str) -> dict:
    """Fetch all summary metrics from the Metrics MCP service (cached 60 s)."""
    total_posts    = call_mcp(base_url, "get_total_posts")
    total_eng      = call_mcp(base_url, "get_total_engagement")
    avg_rate       = call_mcp(base_url, "get_avg_engagement_rate")
    top_sources    = call_mcp(base_url, "get_top_sources", {"limit": 10})
    eng_by_source  = call_mcp(base_url, "get_engagement_by_source")
    pos_count      = call_mcp(base_url, "get_posts_by_sentiment", {"sentiment": "positive"})
    neu_count      = call_mcp(base_url, "get_posts_by_sentiment", {"sentiment": "neutral"})
    neg_count      = call_mcp(base_url, "get_posts_by_sentiment", {"sentiment": "negative"})
    return {
        "total_posts":   total_posts,
        "total_eng":     total_eng,
        "avg_rate":      avg_rate,
        "top_sources":   top_sources,
        "eng_by_source": eng_by_source,
        "sentiment": {
            "Positive": pos_count or 0,
            "Neutral":  neu_count or 0,
            "Negative": neg_count or 0,
        },
    }

# ── Tabs ───────────────────────────────────────────────────────────────────────
st.title("Social Media Interactions Analyzer")
tab1, tab2, tab3 = st.tabs(["💬 Chat", "📊 Metrics", "🗂️ Data"])

# ── Tab 1: Chat ────────────────────────────────────────────────────────────────
with tab1:
    st.header("Ask about your social media data")

    if "messages" not in st.session_state:
        st.session_state.messages = []

    for msg in st.session_state.messages:
        with st.chat_message(msg["role"]):
            st.markdown(msg["content"])

    query = st.chat_input("Ask a question...")

    if query:
        st.session_state.messages.append({"role": "user", "content": query})
        with st.chat_message("user"):
            st.markdown(query)

        with st.chat_message("assistant"):
            with st.spinner("Thinking..."):
                try:
                    response = requests.post(
                        f"{backend_url}/query",
                        json={"query": query},
                        timeout=60,
                    )
                    if response.status_code == 200:
                        answer = response.json().get("answer", "No response")
                        st.markdown(answer)
                        st.session_state.messages.append({"role": "assistant", "content": answer})
                    else:
                        st.error(f"Backend error: {response.status_code}")
                except requests.exceptions.ConnectionError:
                    st.error(f"Cannot connect to backend at {backend_url}")
                except Exception as exc:
                    st.error(f"Error: {exc}")

    if st.sidebar.button("Clear Chat"):
        st.session_state.messages = []
        st.rerun()

# ── Tab 2: Metrics (live data from Metrics MCP) ────────────────────────────────
with tab2:
    st.header("Engagement Metrics")

    with st.spinner("Loading metrics from MCP service…"):
        m = fetch_metrics(metrics_url)

    # KPI row
    col1, col2, col3 = st.columns(3)
    with col1:
        val = f"{m['total_posts']:,}" if m["total_posts"] is not None else "—"
        st.metric("Total Posts", val)
    with col2:
        val = f"{m['total_eng']:,}" if m["total_eng"] is not None else "—"
        st.metric("Total Engagement (Likes)", val)
    with col3:
        val = f"{m['avg_rate']:.2%}" if m["avg_rate"] is not None else "—"
        st.metric("Avg Engagement Rate", val)

    # Posts by source bar chart
    if m["top_sources"]:
        st.subheader("Top Sources by Post Count")
        df_sources = pd.DataFrame(
            list(m["top_sources"].items()), columns=["Source", "Posts"]
        ).sort_values("Posts", ascending=False)
        fig = px.bar(df_sources, x="Source", y="Posts", title="Posts by Source")
        st.plotly_chart(fig, use_container_width=True)
    else:
        st.info("Source data unavailable — is the Metrics MCP running?")

    # Engagement by source bar chart
    if m["eng_by_source"]:
        st.subheader("Engagement by Source")
        df_eng = pd.DataFrame(
            list(m["eng_by_source"].items()), columns=["Source", "Likes"]
        ).sort_values("Likes", ascending=False)
        fig_eng = px.bar(df_eng, x="Source", y="Likes", title="Total Likes by Source", color="Likes")
        st.plotly_chart(fig_eng, use_container_width=True)

    # Sentiment pie chart
    sentiment = m["sentiment"]
    if any(v for v in sentiment.values()):
        st.subheader("Sentiment Distribution")
        df_sent = pd.DataFrame(
            [{"Sentiment": k, "Count": v} for k, v in sentiment.items()]
        )
        fig2 = px.pie(df_sent, values="Count", names="Sentiment", title="Sentiment Distribution")
        st.plotly_chart(fig2, use_container_width=True)
    else:
        st.info("Sentiment data unavailable — is the Metrics MCP running?")

# ── Tab 3: Data Explorer ───────────────────────────────────────────────────────
with tab3:
    st.header("Data Explorer")

    try:
        response = requests.get(f"{backend_url}/posts", timeout=30)
        if response.status_code == 200:
            posts = response.json().get("posts", [])
            if posts:
                df = pd.DataFrame(posts)
                st.dataframe(df.head(50), use_container_width=True)
            else:
                st.info("No posts data available from backend.")
        else:
            st.error(f"Error loading data: {response.status_code}")
    except requests.exceptions.ConnectionError:
        st.error(f"Cannot connect to backend at {backend_url}")
    except Exception as exc:
        st.error(f"Error: {exc}")