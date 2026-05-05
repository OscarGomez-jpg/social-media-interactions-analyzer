
import streamlit as st
import requests
import json
import pandas as pd
import re
import time

# --- Configuración de la Página ---
st.set_page_config(
    page_title="Analizador de Interacciones",
    page_icon="🧠",
    layout="centered",
    initial_sidebar_state="expanded"
)

# --- Estilos CSS Personalizados ---
st.markdown("""
<style>
    /* Estilo para el botón de Tests fijo en la esquina superior izquierda */
    div[data-testid="stButton"] {
        position: fixed !important;
        top: 1rem !important;
        left: 1rem !important;
        width: auto !important;
        margin: 0 !important;
        z-index: 999999 !important;
    }
    .st-emotion-cache-1c7y2kd { /* Contenedor del chat input */
        background-color: #f0f2f6;
    }
    .st-emotion-cache-janbn0 { /* Burbuja de chat del usuario */
        background-color: #d0f0c0; /* Verde claro */
    }
</style>
""", unsafe_allow_html=True)


# --- Cliente JSON-RPC (Sin cambios) ---
def call_mcp(method: str, params: dict = None) -> dict:
    """
    Realiza una llamada JSON-RPC 2.0 a un servicio MCP, manejando respuestas
    JSON directas y text/event-stream (SSE).
    """
    url = "http://localhost:8001/mcp"
    headers = {
        'Content-Type': 'application/json',
        'Accept': 'application/json, text/event-stream'
    }
    
    mcp_params = {"name": method, "arguments": params or {}}
    payload = {"jsonrpc": "2.0", "method": "tools/call", "params": mcp_params, "id": 1}

    try:
        # SOLUCIÓN: Usar stream=True para manejar respuestas SSE
        with requests.post(url, data=json.dumps(payload), headers=headers, timeout=120, stream=True) as response:
            response.raise_for_status()
            
            content_type = response.headers.get('content-type', '')
            
            # Si la respuesta es un flujo de eventos (comportamiento por defecto de FastMCP)
            if 'text/event-stream' in content_type:
                for line in response.iter_lines():
                    if line:
                        decoded_line = line.decode('utf-8')
                        if decoded_line.startswith('data:'):
                            # Extraemos el JSON que viene después de "data: "
                            json_data = decoded_line[len('data:'):].strip()
                            if json_data != "[DONE]":
                                rpc_response = json.loads(json_data)
                                break # Procesamos solo el primer evento de datos
                else: # Si el bucle termina sin un break
                    return {"error": "Stream de eventos finalizado sin datos."}
            
            # Si la respuesta es JSON plano
            elif 'application/json' in content_type:
                rpc_response = response.json()
            
            else:
                return {"error": f"Tipo de contenido no soportado: {content_type}"}

        # --- Lógica de desempaquetado (común para ambos casos) ---
        if 'error' in rpc_response:
            return rpc_response
        
        raw_result = rpc_response.get('result', {})
        content = raw_result.get('content', [])
        if content and 'text' in content[0]:
            return json.loads(content[0]['text'])
        return raw_result

    except requests.exceptions.RequestException as e:
        return {"error": f"Error de conexión: {e}"}
    except json.JSONDecodeError as e:
        return {"error": f"No se pudo decodificar la respuesta JSON: {e}"}
    except Exception as e:
        return {"error": f"Ocurrió un error inesperado: {e}"}

# Lógica de Interpretación de Prompts (NLU simple) ---
def analyze_prompt(prompt: str) -> (str, dict):
    """Interpreta el prompt del usuario para determinar el MCP y los parámetros."""
    prompt_lower = prompt.lower()

    # 1. Análisis de Propagación (busca "post" y un ID)
    propagation_match = re.search(r'(analiza|propagaci.n|alcance|impacto).*(post|id)\s+([a-zA-Z0-9_.-]+)', prompt_lower)
    if propagation_match:
        post_id = propagation_match.group(3)
        return "analyze_propagation", {"post_id": post_id}

    # 2. Resumen (busca palabras clave como "resume", "sentimiento", "clima")
    if any(keyword in prompt_lower for keyword in ["resume", "resumen", "sentimiento", "clima", "tema"]):
        sentiment = None
        if "positivo" in prompt_lower: sentiment = "POSITIVE"
        elif "negativo" in prompt_lower: sentiment = "NEGATIVE"
        elif "neutral" in prompt_lower: sentiment = "NEUTRAL"
        
        params = {"sentiment": sentiment} if sentiment else {}
        return "get_summary", params

    # 3. Métricas (con palabras clave como "métricas", "viral", "influencer")
    # SOLUCIÓN: Extraer el número si se especifica, si no, usar 5.
    
    if any(keyword in prompt_lower for keyword in ["m.tricas", "viral", "influencer", "top", "populares"]):
        limit = 5
        metrics_match = re.search(r'(\d+)\s+(posts|influencers|m.tricas)', prompt_lower)
        if metrics_match:
            try:
                limit = int(metrics_match.group(1))
            except (ValueError, IndexError):
                pass
        query_type = "influencers" if "influencer" in prompt_lower else "posts"
        return "get_metrics", {"limit": limit, "query_type": query_type}

    # Fallback: si no se reconoce, se intenta un resumen general
    return "get_summary", {}

# --- Funciones para mostrar resultados ---
def display_result(result: dict):
    """Formatea y muestra el resultado del MCP en el chat."""
    if "error" in result:
        st.error(f"Ocurrió un error: {result['error']}")
        return

    # Formato para Métricas
    if "viral_posts" in result or "top_influencers" in result:
        if "viral_posts" in result:
            st.markdown("##### 🚀 Posts Más Virales")
            viral_df = pd.DataFrame(result["viral_posts"])
            if not viral_df.empty:
                st.dataframe(viral_df, width="stretch")
            else:
                st.markdown("_No se encontraron posts virales._")

        if "top_influencers" in result:
            st.markdown("##### 👑 Top Influencers")
            influencers_df = pd.DataFrame(result["top_influencers"])
            if not influencers_df.empty:
                st.dataframe(influencers_df, width="stretch")
            else:
                st.markdown("_No se encontraron influencers._")

    # Formato para Resumen
    elif "summary" in result:
        st.markdown(result.get("summary", "_No se recibió contenido en el resumen._"))
        if result.get("method"):
            st.caption(f"Generado con: {result['method']}")

    # Formato para Propagación
    elif "accumulated_reach" in result:
        st.markdown(f"##### Análisis del Post: `{result.get('post_id')}`")
        st.text_area("Texto Original (fragmento)", value=result.get("original_text", ""), height=100, disabled=True)
        
        col1, col2, col3 = st.columns(3)
        col1.metric("Alcance Acumulado", f"{result.get('accumulated_reach', 0):,}")
        col2.metric("Total de Respuestas", f"{result.get('total_replies', 0):,}")
        col3.metric("Puntuación de Impacto", f"{result.get('impact_score', 0):.2f}")

    else:
        st.markdown("##### Respuesta Desconocida")
        st.json(result)

@st.dialog("Preguntas de prueba del sistema", width="large")
def show_test_questions():
    st.markdown("Copia estas preguntas tal cual en el chat. Cada una activa el MCP indicado.")

    st.markdown("#### 📊 `get_metrics` — Posts virales")
    st.code("¿Cuáles son los posts más virales?", language=None)
    st.code("Dame los 3 posts más populares", language=None)
    st.markdown("#### 📊 `get_metrics` — Influencers")
    st.code("Dame los 3 influencers más importantes", language=None)
    st.code("¿Cuáles son los top 5 influencers?", language=None)

    st.markdown("#### 📝 `get_summary` — Resumen con Ollama")
    st.code("Dame un resumen de los posts actuales", language=None)
    st.code("Dame un resumen de los posts positivos", language=None)
    st.code("¿Cuál es el clima de los comentarios negativos?", language=None)

    st.markdown("#### 🌳 `analyze_propagation` — Propagación en árbol")
    st.code("Analiza la propagación del post tikapi_7520805329748151557", language=None)
    st.caption("↑ Hilo más grande del dataset: 2 920 respuestas")
    st.code("Analiza la propagación del post c6adb4630994bdee807d387382d526bc", language=None)
    st.caption("↑ Post sin hijos: alcance esperado = 1")
    st.code("Analiza la propagación del post xyz_no_existe_123", language=None)
    st.caption("↑ ID inexistente: debe devolver error controlado")


# --- Interfaz de Usuario Principal ---

# Botón de tests que se renderizará en la esquina superior derecha gracias al CSS
if st.button("🧪 Tests", help="Ver preguntas de prueba del sistema"):
    show_test_questions()

st.title("Analizador de Interacciones")

st.markdown("Chatea con la IA para analizar datos de redes sociales. El sistema elegirá el MCP adecuado por ti.")

# Inicialización del historial de chat
if "messages" not in st.session_state:
    st.session_state.messages = []

# Mostrar mensajes previos
for message in st.session_state.messages:
    with st.chat_message(message["role"]):
        # Usamos diferentes funciones para mostrar el contenido
        if isinstance(message["content"], dict):
            display_result(message["content"])
        else:
            st.markdown(message["content"])

# Input del usuario
if prompt := st.chat_input("¿Qué quieres analizar hoy?"):
    # Añadir y mostrar el mensaje del usuario
    st.session_state.messages.append({"role": "user", "content": prompt})
    with st.chat_message("user"):
        st.markdown(prompt)

    # Procesar la respuesta del asistente
    with st.chat_message("assistant"):
        # 1. "Pensamiento" del asistente
        with st.spinner("Pensando..."):
            mcp_method, mcp_params = analyze_prompt(prompt)
            thought = f"🤔 **Pensamiento:** He interpretado la petición y voy a activar el MCP `{mcp_method}`"
            if mcp_params:
                thought += f" con los parámetros `{mcp_params}`."
            else:
                thought += "."
            st.markdown(thought)
            st.session_state.messages.append({"role": "assistant", "content": thought})

        # 2. Llamada al MCP y muestra de resultados
        with st.spinner(f"Contactando al MCP `{mcp_method}`..."):
            time.sleep(1) # Pequeña pausa para mejorar la UX
            response_data = call_mcp(mcp_method, mcp_params)
            
            # Mostrar y guardar el resultado
            display_result(response_data)
            st.session_state.messages.append({"role": "assistant", "content": response_data})

