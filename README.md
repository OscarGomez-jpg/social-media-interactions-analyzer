# Social Media Interactions Analyzer (Simplified)

Sistema multi-agente para análisis de interacciones en redes sociales.

## Requisitos
- Go 1.23+
- Python 3.11+
- Ollama (modelo `gemma4:e2b`)

## Configuración
1. Coloca tu archivo `data.parquet` en `db/data.parquet`.
2. El archivo `.env` ya está configurado para Ollama.

## Ejecución (2 Terminales)

### Terminal 1: Servicio MCP
```bash
cd mcp_services
source venv/bin/activate
pip install -r requirements.txt
python all_in_one_mcp.py
```

### Terminal 2: Agente Go
```bash
go run .
```

### Web Streamlit
```bash
streamlit run app.py
```

## Estructura Simplificada
- `main.go`: Orquestador principal.
- `mcp_services/all_in_one_mcp.py`: Microservicio unificado con todas las herramientas.
- `pkg/`: Lógica interna (cliente MCP, LLM, etc).
- `db/`: Directorio para el dataset Parquet.
