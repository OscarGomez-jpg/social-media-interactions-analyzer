Integrantes:

- David Henao
- Juan Manuel Marín
- Johan Daniel Aguirre
- Óscar Andrés Gómez

Este es un reto clásico de **Social Listening** (Escucha Social) llevado al siguiente nivel mediante el uso de **Agentes de IA** y el estándar **MCP (Model Context Protocol)**. 

Básicamente, el problema no es solo analizar datos, sino crear una "capa de inteligencia" que traduzca las preguntas abstractas de un humano (ej. *"¿Cómo va la cosa en Twitter?"*) en llamadas técnicas a microservicios específicos.

---

## 1. Contextualización del Problema
Estás construyendo un sistema de **Soporte de Decisiones**. El dataset contiene el "caos" de las redes sociales. Tu misión es transformar ese caos en **señales claras** mediante tres capas:
1.  **Datos Raw:** El JSON/CSV de publicaciones.
2.  **Cerebros Analíticos (MCPs):** Microservicios que solo saben hacer una cosa muy bien (contar likes, resumir texto o rastrear un link).
3.  **El Orquestador (Agente):** El "Project Manager" que recibe la orden y decide a qué microservicio llamar.

---

## 2. Estrategia Ganadora: Selección de Servicios
Para maximizar el impacto y la facilidad de implementación, te sugiero elegir estos dos adicionales al obligatorio:

* **Opción A: Resumen General de la Conversación (LLM):** Es fundamental. Sin un resumen, el usuario tiene métricas pero no "contexto". Es el que mejor luce en una demo.
* **Opción B: Análisis de Métricas (Procesamiento Tradicional):** Es "barato" en términos de cómputo (no requiere LLM) y proporciona datos duros (influencers, posts más virales) que validan la parte cualitativa del resumen.

### ¿Por qué esta combinación?
Junto con el de **Propagación (Obligatorio)**, cubres el espectro completo:
1.  **Qué** se dice (Resumen).
2.  **Quién** es importante (Métricas).
3.  **Cómo** se mueve el mensaje (Propagación).

---

## 3. Optimización Costo-Beneficio
En proyectos de IA, el costo suele dispararse en los **Tokens de entrada** y el **Tiempo de desarrollo**.

* **Usa Modelos "Pequeños" para Tareas Específicas:** No uses GPT-4o o Gemini 1.5 Pro para clasificar sentimientos o contar likes. Usa modelos como `gpt-4o-mini` o `gemini-1.5-flash`. Son 10 veces más baratos y para análisis de texto corto son igual de efectivos.
* **Cache de Resultados:** Si el dataset no cambia en tiempo real durante la demo, guarda el resultado del análisis en una pequeña base de datos o incluso en memoria (Redis/Diccionario Python). Si el usuario pregunta dos veces lo mismo, no gastes tokens de nuevo.
* **Pre-procesamiento de Texto:** Limpia el ruido (URLs repetidas, stop words, emojis excesivos) antes de enviar el texto al LLM. Menos texto = menos costo.

---

## 4. Optimización Tiempo-Beneficio (Velocidad de Entrega)
Para ganar el reto, necesitas ser ágil en la implementación:

### El Stack de "Vía Rápida":
* **FastMCP:** Es una librería que permite convertir funciones de Python en servidores MCP casi instantáneamente. Te ahorrará horas de configurar rutas en FastAPI manualmente.
* **LangGraph (Para los puntos extra):** No te compliques con arquitecturas circulares complejas. Define un grafo lineal simple: 
    * `Estado Inicial` $\rightarrow$ `Nodo de Decisión (Tool Calling)` $\rightarrow$ `Ejecución de Herramienta` $\rightarrow$ `Generación de Respuesta`.
* **Pandas para Métricas:** No reinventes la rueda. Para el MCP de métricas y propagación, usa la librería `pandas` para filtrar y agrupar los datos en 3 líneas de código.

---

## 5. Hoja de Ruta Sugerida (Action Plan)

| Fase | Tarea Crítica | Tip Pro |
| :--- | :--- | :--- |
| **Data** | Identificar la jerarquía `post_id` vs `reply_to`. | Sin esto, el análisis de **Propagación** fallará. |
| **MCPs** | Crear los endpoints con `FastAPI`. | Devuelve siempre el JSON más simple posible para que el Agente no se confunda. |
| **Agente** | Configurar el `System Prompt`. | Dile al agente: *"Eres un analista experto. Si no tienes datos de propagación, no inventes, llama a la herramienta."* |
| **Demo** | Interfaz con `Streamlit`. | Es la forma más rápida (en Python) de crear una interfaz web donde el usuario chatee con el agente. |

### El Factor Diferenciador:
El análisis de **Propagación** suele ser el más difícil. Para que sea rentable en tiempo, modela los datos como un **árbol**.
$$Alcance = \sum (\text{Likes} + \text{Replies}) \text{ de todos los nodos descendientes}$$
Si logras visualizar esto en una tabla simple o un gráfico de barras, tienes el éxito asegurado.
