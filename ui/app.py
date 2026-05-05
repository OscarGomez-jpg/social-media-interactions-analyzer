import streamlit as st
import requests
import json
import pandas as pd
import plotly.express as px

BACKEND_URL = st.session_state.get("backend_url", "http://localhost:8080")

st.set_page_config(page_title="Social Media Analyzer", layout="wide")

st.title("Social Media Interactions Analyzer")

st.sidebar.header("Configuration")
backend_url = st.sidebar.text_input("Backend URL", value=BACKEND_URL, key="backend_url_input")
st.session_state.backend_url = backend_url

tab1, tab2, tab3 = st.tabs(["Chat", "Metrics", "Data"])

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
                        timeout=60
                    )
                    if response.status_code == 200:
                        answer = response.json().get("answer", "No response")
                        st.markdown(answer)
                        st.session_state.messages.append({"role": "assistant", "content": answer})
                    else:
                        st.error(f"Error: {response.status_code}")
                except requests.exceptions.ConnectionError:
                    st.error(f"Cannot connect to backend at {backend_url}")
                except Exception as e:
                    st.error(f"Error: {str(e)}")

with tab2:
    st.header("Engagement Metrics")
    
    col1, col2, col3 = st.columns(3)
    
    with col1:
        st.metric("Total Posts", "4,795")
    with col2:
        st.metric("Total Engagement", "12,450")
    with col3:
        st.metric("Avg Engagement Rate", "2.6%")
    
    st.subheader("Engagement by Source")
    
    sources_data = {
        "Source": ["Noticias Caracol", "Blu Radio", "Other"],
        "Posts": [197, 130, 4468]
    }
    df_sources = pd.DataFrame(sources_data)
    fig = px.bar(df_sources, x="Source", y="Posts", title="Posts by Source")
    st.plotly_chart(fig, use_container_width=True)
    
    st.subheader("Sentiment Distribution")
    
    sentiment_data = {
        "Sentiment": ["Positive", "Neutral", "Negative"],
        "Count": [1800, 2200, 795]
    }
    df_sentiment = pd.DataFrame(sentiment_data)
    fig2 = px.pie(df_sentiment, values="Count", names="Sentiment", title="Sentiment Distribution")
    st.plotly_chart(fig2, use_container_width=True)

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
                st.info("No posts data available")
        else:
            st.error(f"Error loading data: {response.status_code}")
    except requests.exceptions.ConnectionError:
        st.error(f"Cannot connect to backend at {backend_url}")
    except Exception as e:
        st.error(f"Error: {str(e)}")

if st.sidebar.button("Clear Chat"):
    st.session_state.messages = []
    st.rerun()