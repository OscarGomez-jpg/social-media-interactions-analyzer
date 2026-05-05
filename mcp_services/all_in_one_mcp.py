import os
import re
import subprocess
import pandas as pd
from collections import defaultdict
from pathlib import Path
from fastmcp import FastMCP

try:
    from dotenv import load_dotenv
    load_dotenv(Path(__file__).parent.parent / ".env")
except ImportError:
    pass

mcp = FastMCP("Social Media Analyzer Unified")


def load_data() -> pd.DataFrame:
    base_dir = os.path.dirname(os.path.abspath(__file__))
    abs_path = os.path.join(base_dir, "..", "db", "data.parquet")

    if not os.path.exists(abs_path):
        print(f"❌ Error crítico: No se encontró el archivo en {abs_path}")
        return pd.DataFrame()

    try:
        print(f"✅ Cargando datos desde: {abs_path}")
        df = pd.read_parquet(abs_path)
        if "id" in df.columns:
            df["id"] = df["id"].astype(str)
        if "parentId" in df.columns:
            df["parentId"] = df["parentId"].astype(str).replace("nan", None)
        return df
    except Exception as e:
        print(f"Error loading data: {e}")
        return pd.DataFrame()


def clean_text(text: str) -> str:
    if pd.isna(text):
        return ""
    text = str(text)
    text = re.sub(r'http\S+|www\.\S+|@\w+|#\w+', '', text)
    text = re.sub(r'[^\w\s.,!?]', '', text)
    return re.sub(r'\s+', ' ', text).strip()


def _ollama_base() -> str:
    """Returns Ollama base URL. Reads OLLAMA_HOST env var, otherwise auto-detects WSL2 gateway."""
    host = os.getenv("OLLAMA_HOST")
    if host:
        return host.rstrip("/")
    try:
        out = subprocess.run(
            ["ip", "route", "show", "default"],
            capture_output=True, text=True, timeout=2
        ).stdout
        parts = out.split()
        if "via" in parts:
            return f"http://{parts[parts.index('via') + 1]}:11434"
    except Exception:
        pass
    return "http://localhost:11434"


# --- MÓDULO 1: MÉTRICAS ---
@mcp.tool()
def get_metrics(limit: int = 5, query_type: str = "posts") -> dict:
    """Identifica posts virales (query_type='posts') o usuarios influyentes (query_type='influencers')."""
    print(f"📊 [MCP] Llamada a get_metrics(limit={limit}, query_type={query_type})")
    df = load_data()
    if df.empty:
        return {"error": "No data available"}

    df["liked"] = pd.to_numeric(df["liked"], errors="coerce").fillna(0)

    if query_type == "influencers":
        if "sourceName" not in df.columns:
            return {"top_influencers": []}
        source_df = df[
            df["sourceName"].notna() & (df["sourceName"].astype(str).str.strip() != "")
        ]
        influencers = (
            source_df.assign(liked_num=source_df["liked"])
            .groupby("sourceName")["liked_num"]
            .sum()
            .sort_values(ascending=False)
            .head(limit)
        )
        influencer_list = [{"user": k, "total_likes": int(v)} for k, v in influencers.items()]
        print(f"✅ [MCP] get_metrics: {len(influencer_list)} influencers")
        return {"top_influencers": influencer_list}

    # default: posts
    viral_posts = df.sort_values(by="liked", ascending=False).head(limit)
    print(f"✅ [MCP] get_metrics: {len(viral_posts)} posts virales")
    return {
        "viral_posts": viral_posts[["id", "text", "liked", "engagementRate"]].to_dict(orient="records"),
    }


# --- MÓDULO 2: RESUMEN GENERAL ---
@mcp.tool()
def get_summary(sentiment: str = None) -> dict:
    """Sintetiza el clima de la conversación usando Ollama local."""
    print(f"📝 [MCP] Llamada a get_summary(sentiment={sentiment})")
    df = load_data()
    if df.empty:
        return {"available": False, "error": "No hay datos"}

    filtered = df[df["sentiment"] == sentiment] if sentiment else df
    texts = [clean_text(t) for t in filtered["text"].head(6) if pd.notna(t)]
    if not texts:
        return {"available": False, "error": "No se encontraron posts"}

    print(f"🔄 [MCP] get_summary: Procesando {len(texts)} textos...")

    prompt = (
        "En máximo 2 párrafos, resume las temáticas principales y el clima general "
        "de estos posts de redes sociales. Sé directo y no uses listas numeradas:\n"
        + "\n".join([f"- {t[:150]}" for t in texts])
    )

    try:
        import requests
        ollama_url = f"{_ollama_base()}/api/chat"
        print(f"🦙 [MCP] get_summary: Llamando a Ollama en {ollama_url}")
        resp = requests.post(ollama_url, json={
            "model": "gemma4:e2b",
            "messages": [{"role": "user", "content": prompt}],
            "stream": False,
            "think": False,
            "options": {"num_predict": 350},
        }, timeout=90)
        if resp.status_code == 200:
            data = resp.json()
            msg = data.get("message", {})
            summary = msg.get("content", "") or data.get("response", "")
            return {"available": True, "summary": summary, "method": "ollama_local"}
        return {"available": False, "error": f"Ollama returned status {resp.status_code}"}
    except Exception as e:
        return {"available": False, "error": f"Error en resumen (Ollama): {str(e)}"}


# --- MÓDULO 3: ANÁLISIS DE PROPAGACIÓN ---
@mcp.tool()
def analyze_propagation(post_id: str) -> dict:
    """
    Calcula el alcance acumulado usando lógica de árbol.
    Alcance = sum(Likes + 1) de todos los nodos descendientes.
    """
    print(f"🌳 [MCP] Llamada a analyze_propagation(post_id={post_id})")
    df = load_data()
    if df.empty:
        return {"error": "No data"}

    df["liked"] = pd.to_numeric(df["liked"], errors="coerce").fillna(0)

    children_map = defaultdict(list)
    post_data = {}
    for _, row in df.iterrows():
        pid = str(row["id"])
        parent = str(row["parentId"]) if pd.notna(row["parentId"]) else None
        children_map[parent].append(pid)
        post_data[pid] = {"likes": int(row["liked"]), "text": str(row.get("text", ""))}

    if post_id not in post_data:
        print(f"❌ [MCP] analyze_propagation: Post {post_id} no encontrado")
        return {"error": f"Post {post_id} no encontrado"}

    total_reach = 0
    nodes_visited = 0
    queue = [post_id]
    visited = set()

    while queue:
        curr = queue.pop(0)
        if curr in visited:
            continue
        visited.add(curr)
        total_reach += (post_data[curr]["likes"] + 1)
        nodes_visited += 1
        queue.extend(children_map.get(curr, []))

    print(f"✅ [MCP] analyze_propagation: Alcance={total_reach}, Nodos={nodes_visited}")

    return {
        "post_id": post_id,
        "original_text": post_data[post_id]["text"][:100],
        "accumulated_reach": total_reach,
        "total_replies": nodes_visited - 1,
        "impact_score": total_reach * 1.5,
    }


if __name__ == "__main__":
    mcp.run(transport="http", port=8001, stateless_http=True)
