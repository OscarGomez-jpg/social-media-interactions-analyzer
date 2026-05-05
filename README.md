# Social Media Interactions Analyzer

> Sistema multi-agente que transforma preguntas en lenguaje natural en insights accionables sobre interacciones en redes sociales.

## Tabla de Contenidos

- [Arquitectura](#arquitectura)
- [Requisitos previos](#requisitos-previos)
- [Configuración](#configuración)
- [Ejecución](#ejecución)
- [Estructura del proyecto](#estructura-del-proyecto)
- [Equipo](#equipo)

---

## Arquitectura

```
┌─────────────────────────────────────────────────────────────┐
│  PRESENTATION                                               │
│  Streamlit UI (ui/app.py)  ·  Terminal interactivo          │
└────────────────────────┬────────────────────────────────────┘
                         │  HTTP POST /query
┌────────────────────────▼────────────────────────────────────┐
│  ORCHESTRATION                                              │
│  Agente Go – ReAct loop          Caché SHA-256 (2 capas)   │
│  main.go  ·  pkg/llm/ollama.go   in-memory, query + MCP    │
└───────────────┬──────────────────────────────────────────── ┘
                │  JSON-RPC 2.0  ·  pkg/mcp/client.go
                │  (timeout 10 s, errores estructurados)
┌───────────────▼─────────────────────────────────────────────┐
│  MCP SERVICES  (Python · FastMCP · stdio)                   │
│  metrics_mcp.py :8001   propagation_mcp.py :8002            │
│  summary_mcp.py :8003                                       │
└───────────────┬─────────────────────────────────────────────┘
                │  pandas.read_parquet
┌───────────────▼─────────────────────────────────────────────┐
│  DATA                                                       │
│  db/data.parquet  (fuente única, sólo lectura)              │
└─────────────────────────────────────────────────────────────┘
```

### Flujo de una consulta

1. El **usuario** escribe una pregunta en lenguaje natural.
2. El **agente Go** consulta el hash SHA-256 de la pregunta en su caché; si hay hit, responde al instante.
3. Si no hay caché, Ollama (`gemma4:e2b` corriendo localmente) decide qué herramienta(s) invocar.
4. El agente llama al **MCP correspondiente** vía JSON-RPC 2.0; cada MCP lee `db/data.parquet` con pandas.
5. El agente sintetiza los resultados con Ollama y almacena la respuesta en caché.
6. La respuesta llega al usuario (terminal o Streamlit).

### Microservicios MCP

| Servicio | Puerto | Tipo | Descripción |
|---|---|---|---|
| `metrics_mcp.py` | 8001 | Determinista | Total posts, likes, engagement rate, top fuentes, posts por sentimiento |
| `propagation_mcp.py` | 8002 | Determinista | Árbol de replies (BFS), alcance acumulado, top posts por reach |
| `summary_mcp.py` | 8003 | LLM (GPT-4o-mini) | Resumen temático de posts por sentimiento; limpieza de URLs/emojis |

---

## Requisitos Previos

### Obligatorios

| Herramienta | Versión mínima | Propósito |
|---|---|---|
| [Go](https://go.dev/dl/) | 1.23 | Agente orquestador |
| [Python](https://www.python.org/downloads/) | 3.11 | Servicios MCP |
| [Ollama](https://ollama.com/) | cualquiera | LLM local (backend principal) |

### Opcionales

| Herramienta | Propósito |
|---|---|
| Clave de API de [OpenAI](https://platform.openai.com/) | Activar `summary_mcp` con GPT-4o-mini |
| [Streamlit](https://streamlit.io/) | Interfaz web (ya incluido en `requirements.txt`) |

---

## Configuración

### 1. Clonar el repositorio

```bash
git clone https://github.com/OscarGomez-jpg/social-media-interactions-analyzer.git
cd social-media-interactions-analyzer
git checkout dev
```

### 2. Datos

El dataset (`db/data.parquet`) **no está incluido en el repositorio** por su tamaño. Coloca el archivo en:

```
social-media-interactions-analyzer/
└── db/
    └── data.parquet   ← aquí
```

> El archivo debe tener al menos las columnas: `id`, `parentId`, `text`, `liked`, `engagementRate`, `sourceName`, `sentiment`.

### 3. Variables de entorno

```bash
cp .env.example .env
```

Edita `.env`:

```env
# Backend LLM — debe ser "ollama"
LLM_BACKEND=ollama

# Activar microservicios MCP (true | false)
USE_MCP=true

# Sólo necesaria si vas a usar summary_mcp con GPT-4o-mini
OPENAI_API_KEY=sk-...
```

### 4. Modelo Ollama

```bash
# Descargar el modelo que usa el agente Go
ollama pull gemma4:e2b
```

---

## Ejecución

Abre **4 terminales** desde la raíz del proyecto.

### Terminal 1 — Metrics MCP

```bash
cd mcp_services

# Primera vez: crear entorno virtual e instalar dependencias
python -m venv venv
venv\Scripts\activate        # Windows
# source venv/bin/activate   # macOS / Linux

pip install -r requirements.txt

# Iniciar servicio
python metrics_mcp.py
```

### Terminal 2 — Propagation MCP

```bash
cd mcp_services
venv\Scripts\activate
python propagation_mcp.py
```

### Terminal 3 — Summary MCP

```bash
cd mcp_services
venv\Scripts\activate
python summary_mcp.py
```

> Si no configuras `OPENAI_API_KEY`, las llamadas a `summarize_posts_by_sentiment` devolverán `{"available": false, "error": "OPENAI_API_KEY is not configured"}`. El resto de herramientas funcionan igual.

### Terminal 4 — Agente Go

```bash
# Compilar y ejecutar
go run .
```

El agente iniciará el loop interactivo en la terminal. Escribe cualquier pregunta en español, por ejemplo:

```
You: ¿Cuántos posts hay en el dataset?
You: ¿Cuál es la tasa de engagement promedio?
You: Muéstrame los posts con mayor alcance
You: quit
```

---

## Interfaz Web (Streamlit) — Opcional

En una quinta terminal:

```bash
cd mcp_services
venv\Scripts\activate
cd ..
streamlit run ui/app.py
```

Abre [http://localhost:8501](http://localhost:8501) en tu navegador.

La UI tiene tres secciones:
- **Chat** — Pregunta al agente Go en lenguaje natural (requiere el agente corriendo).
- **Metrics** — Dashboard en vivo con KPIs y gráficas cargadas desde `metrics_mcp` en tiempo real.
- **Data** — Explorador de posts (requiere el agente corriendo con endpoint `/posts`).

> **Nota:** El tab "Metrics" llama directamente a `localhost:8001` (metrics_mcp). El tab "Chat" llama a `localhost:8080` (agente Go).

---

## Estructura del Proyecto

```
social-media-interactions-analyzer/
│
├── main.go                  # Agente Go — ReAct loop, caché, orquestación
├── go.mod / go.sum          # Módulo Go
├── .env.example             # Plantilla de variables de entorno
│
├── pkg/
│   ├── mcp/
│   │   └── client.go        # Cliente JSON-RPC 2.0 con timeout y caché
│   ├── llm/
│   │   └── ollama.go        # Cliente Ollama (tool calling + respuesta final)
│   ├── tools/
│   │   └── tools.go         # Herramientas locales (fallback sin MCP)
│   ├── models/
│   │   └── models.go        # Tipos compartidos (ToolResult, etc.)
│   └── data/
│       └── store.go         # Carga de datos mock (sin MCP)
│
├── mcp_services/
│   ├── requirements.txt     # fastmcp, pandas, pyarrow, openai, plotly, streamlit
│   ├── metrics_mcp.py       # Servicio de métricas — puerto 8001
│   ├── propagation_mcp.py   # Análisis de propagación — puerto 8002
│   └── summary_mcp.py       # Resumen LLM — puerto 8003
│
├── ui/
│   └── app.py               # Dashboard Streamlit
│
├── db/
│   └── data.parquet         # Dataset (no incluido en el repo — ver sección Datos)
│
└── docs/
    └── slides/              # Presentación del proyecto (HTML)
```

---

## Dependencias Clave

### Go
| Paquete | Uso |
|---|---|
| `github.com/joho/godotenv` | Cargar `.env` en el proceso Go |

### Python
| Paquete | Uso |
|---|---|
| `fastmcp` | Framework para exponer herramientas MCP |
| `pandas` + `pyarrow` | Lectura y procesamiento de `data.parquet` |
| `openai` | Llamadas a GPT-4o-mini en `summary_mcp` |
| `streamlit` | Interfaz web |
| `plotly` | Gráficas interactivas en el dashboard |

---

## Equipo

| Nombre | GitHub |
|---|---|
| David Henao | [@NightParker725](https://github.com/NightParker725) |
| Juan Manuel Marín Angarita | [@JMMA86](https://github.com/JMMA86) |
| Johan Daniel Aguirre | [@JohanDanielAguirre](https://github.com/JohanDanielAguirre) |
| Óscar Andrés Gómez | [@OscarGomez-jpg](https://github.com/OscarGomez-jpg) |

---

## Licencia

Proyecto académico — Universidad Icesi · 2026
