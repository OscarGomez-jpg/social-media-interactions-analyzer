#!/bin/bash

# Social Media Interactions Analyzer - Execution Script

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Iniciando Social Media Interactions Analyzer...${NC}"

# 1. Verificar base de datos
if [ ! -f "db/data.parquet" ]; then
    echo -e "${RED}❌ Error: No se encontró db/data.parquet${NC}"
    echo "Por favor, coloca el archivo de datos en la carpeta 'db/' antes de continuar."
    exit 1
fi

# 2. Iniciar Servicio MCP en segundo plano
echo -e "${GREEN}🔌 Limpiando puertos e iniciando Servicio MCP (Python)...${NC}"
# Matar cualquier proceso previo en el puerto 8001
fuser -k 8001/tcp > /dev/null 2>&1
sleep 1

cd mcp_services

if [ ! -d "venv" ]; then
    echo "Creando entorno virtual Python..."
    python3 -m venv venv
fi

source venv/bin/activate
pip install -q -r requirements.txt

# Ejecutar en segundo plano y guardar el PID
python all_in_one_mcp.py > mcp.log 2>&1 &
MCP_PID=$!

# Función para limpiar procesos al salir
cleanup() {
    echo -e "\n${RED}🛑 Deteniendo servicios...${NC}"
    kill $MCP_PID 2>/dev/null
    exit
}

# Atrapar señales de salida
trap cleanup SIGINT SIGTERM

# Esperar a que el servicio MCP arranque
sleep 5
cd ..

# 3. Iniciar Agente Go
echo -e "${GREEN}🤖 Iniciando Agente Interactivo (Go)...${NC}"
go run .

# Al cerrar Go, ejecutar cleanup
cleanup
