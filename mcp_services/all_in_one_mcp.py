import os
import re
import pandas as pd
from collections import defaultdict
from fastmcp import FastMCP

mcp = FastMCP("Social Media Analyzer Unified")
DATA_PATH = "db/data.parquet"

def load_data() -> pd.DataFrame:
    # Obtener la ruta absoluta del directorio donde está este script
    base_dir = os.path.dirname(os.path.abspath(__file__))
    # La base de datos está en ../db/data.parquet respecto a este script
    abs_path = os.path.join(base_dir, "..", "db", "data.parquet")
    
    if not os.path.exists(abs_path):
        print(f"❌ Error crítico: No se encontró el archivo en {abs_path}")
        return pd.DataFrame()

    try:
        print(f"✅ Cargando datos desde: {abs_path}")
        df = pd.read_parquet(abs_path)
        # Asegurar tipos correctos para IDs
        if "id" in df.columns: df["id"] = df["id"].astype(str)
        if "parentId" in df.columns: df["parentId"] = df["parentId"].astype(str).replace("nan", None)
        return df
    except Exception as e:
        print(f"Error loading {DATA_PATH}: {e}")
        return pd.DataFrame()

def clean_text(text: str) -> str:
    if pd.isna(text): return ""
    text = str(text)
    # Limpieza de ruido (URLs, emojis, menciones) según Fase 1.3
    text = re.sub(r'http\S+|www\.\S+|@\w+|#\w+', '', text)
    text = re.sub(r'[^\w\s.,!?]', '', text) # Eliminar emojis excesivos
    return re.sub(r'\s+', ' ', text).strip()

# --- MÓDULO 1: MÉTRICAS (Determinista) ---
@mcp.tool()
def get_metrics(limit: int = 5) -> dict:
    """Identifica posts virales y usuarios influyentes basados en likes y engagement."""
    print(f"📊 [MCP] Llamada a get_metrics(limit={limit})")
    df = load_data()
    if df.empty: 
        print("⚠️ [MCP] get_metrics: DataFrame vacío")
        return {"error": "No data available"}
    
    # Posts virales
    viral_posts = df.sort_values(by="liked", ascending=False).head(limit)
    print(f"✅ [MCP] get_metrics: Encontrados {len(viral_posts)} posts virales")
    
    # Usuarios influyentes (por suma de likes en sus posts)
    if "sourceName" in df.columns:
        influencers = df.groupby("sourceName")["liked"].sum().sort_values(ascending=False).head(limit)
        influencer_list = [{"user": k, "total_likes": int(v)} for k, v in influencers.items()]
    else:
        influencer_list = []

    return {
        "viral_posts": viral_posts[["id", "text", "liked", "engagementRate"]].to_dict(orient="records"),
        "top_influencers": influencer_list
    }

# --- MÓDULO 2: RESUMEN GENERAL (Cualitativo) ---
@mcp.tool()
def get_summary(sentiment: str = None) -> dict:
    """Sintetiza el clima de la conversación usando un modelo destilado (OpenAI o Ollama local)."""
    print(f"📝 [MCP] Llamada a get_summary(sentiment={sentiment})")
    df = load_data()
    if df.empty: 
        print("⚠️ [MCP] get_summary: DataFrame vacío")
        return {"available": False, "error": "No hay datos"}
    
    filtered = df[df["sentiment"] == sentiment] if sentiment else df
    texts = [clean_text(t) for t in filtered["text"].head(15) if pd.notna(t)]
    if not texts: 
        print(f"⚠️ [MCP] get_summary: No se encontraron posts para el sentimiento {sentiment}")
        return {"available": False, "error": "No se encontraron posts"}
    
    print(f"🔄 [MCP] get_summary: Procesando {len(texts)} textos para resumen...")
    
    prompt = f"Resume las temáticas principales y el clima de estos posts de redes sociales:\n" + "\n".join([f"- {t[:200]}" for t in texts])
    
    api_key = os.getenv("OPENAI_API_KEY")
    if api_key:
        try:
            from openai import OpenAI
            client = OpenAI(api_key=api_key)
            response = client.chat.completions.create(model="gpt-4o-mini", messages=[{"role": "user", "content": prompt}])
            return {"available": True, "summary": response.choices[0].message.content}
        except Exception as e:
            print(f"OpenAI error: {e}, falling back to Ollama...")

    # Fallback a Ollama local (siguiendo estrategia de modelos 'Small' del plan)
    try:
        import requests
        resp = requests.post("http://localhost:11434/api/generate", json={
            "model": "gemma4:e2b",
            "prompt": prompt,
            "stream": False
        })
        if resp.status_code == 200:
            return {"available": True, "summary": resp.json().get("response", ""), "method": "ollama_local"}
    except Exception as e:
        return {"available": False, "error": f"Error en resumen (OpenAI/Ollama): {str(e)}"}

# --- MÓDULO 3: ANÁLISIS DE PROPAGACIÓN (OBLIGATORIO) ---
@mcp.tool()
def analyze_propagation(post_id: str) -> dict:
    """
    Calcula el alcance acumulado usando lógica de árbol.
    Alcance = sum(Likes + 1) de todos los nodos descendientes.
    """
    print(f"🌳 [MCP] Llamada a analyze_propagation(post_id={post_id})")
    df = load_data()
    if df.empty: 
        print("⚠️ [MCP] analyze_propagation: DataFrame vacío")
        return {"error": "No data"}
    
    # Construir mapeo relacional
    children_map = defaultdict(list)
    post_data = {}
    for _, row in df.iterrows():
        pid = str(row["id"])
        parent = str(row["parentId"]) if pd.notna(row["parentId"]) else None
        children_map[parent].append(pid)
        post_data[pid] = {"likes": int(row.get("liked", 0)), "text": str(row.get("text", ""))}

    if post_id not in post_data:
        print(f"❌ [MCP] analyze_propagation: Post {post_id} no encontrado en el dataset")
        return {"error": f"Post {post_id} no encontrado"}

    # Recorrido del árbol (BFS) para calcular alcance acumulado
    total_reach = 0
    nodes_visited = 0
    queue = [post_id]
    visited = set()
    
    while queue:
        curr = queue.pop(0)
        if curr in visited: continue
        visited.add(curr)
        
        total_reach += (post_data[curr]["likes"] + 1)
        nodes_visited += 1
        queue.extend(children_map.get(curr, []))

    print(f"✅ [MCP] analyze_propagation: Alcance calculado: {total_reach} sobre {nodes_visited} nodos.")

    return {
        "post_id": post_id,
        "original_text": post_data[post_id]["text"][:100],
        "accumulated_reach": total_reach,
        "total_replies": nodes_visited - 1,
        "impact_score": total_reach * 1.5 # Ejemplo de métrica de impacto mediático
    }

if __name__ == "__main__":
    mcp.run(transport="http", port=8001)
