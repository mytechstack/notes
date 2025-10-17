# AI Conversational Platform - Simple Foundation
# Core components: Conversation Orchestration, Knowledge Management, 
# Prompt Manager, Context Store, Session Manager

from fastapi import FastAPI, HTTPException, Depends
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
from typing import List, Optional, Dict, Any
from datetime import datetime, timedelta
from enum import Enum
import uuid
import json
from collections import defaultdict

# ============================================================================
# Models & Schemas
# ============================================================================

class ConversationState(str, Enum):
    ACTIVE = "ACTIVE"
    PAUSED = "PAUSED"
    ENDED = "ENDED"

class MessageRole(str, Enum):
    USER = "user"
    ASSISTANT = "assistant"
    SYSTEM = "system"

class Message(BaseModel):
    id: str = Field(default_factory=lambda: f"msg_{uuid.uuid4().hex[:12]}")
    role: MessageRole
    content: str
    created_at: datetime = Field(default_factory=datetime.utcnow)
    metadata: Dict[str, Any] = Field(default_factory=dict)

class ConversationConfig(BaseModel):
    model: str = "gpt-3.5-turbo"
    temperature: float = 0.7
    max_tokens: int = 1024
    enable_rag: bool = False
    timeout_seconds: int = 300

class CreateConversationRequest(BaseModel):
    application_id: str
    user_id: Optional[str] = None
    config: Optional[ConversationConfig] = None
    metadata: Dict[str, Any] = Field(default_factory=dict)

class SendMessageRequest(BaseModel):
    content: str
    metadata: Dict[str, Any] = Field(default_factory=dict)

class Conversation(BaseModel):
    id: str
    application_id: str
    user_id: Optional[str]
    state: ConversationState
    created_at: datetime
    updated_at: datetime
    expires_at: datetime
    messages: List[Message] = Field(default_factory=list)
    context: Dict[str, Any] = Field(default_factory=dict)
    config: ConversationConfig

class PromptTemplate(BaseModel):
    id: str
    name: str
    template: str
    variables: List[str]
    version: str = "1.0.0"
    created_at: datetime = Field(default_factory=datetime.utcnow)

class KnowledgeDocument(BaseModel):
    id: str
    title: str
    content: str
    category: str
    application_id: str
    metadata: Dict[str, Any] = Field(default_factory=dict)
    created_at: datetime = Field(default_factory=datetime.utcnow)

class SearchRequest(BaseModel):
    query: str
    application_id: str
    top_k: int = 5

# ============================================================================
# Core Services
# ============================================================================

class SessionManager:
    """Manages conversation sessions and lifecycle"""
    
    def __init__(self):
        self.sessions: Dict[str, Conversation] = {}
        self.user_sessions: Dict[str, List[str]] = defaultdict(list)
    
    def create_session(
        self, 
        application_id: str, 
        user_id: Optional[str] = None,
        config: Optional[ConversationConfig] = None,
        metadata: Dict[str, Any] = None
    ) -> Conversation:
        """Create a new conversation session"""
        session_id = f"conv_{uuid.uuid4().hex[:16]}"
        now = datetime.utcnow()
        
        conversation = Conversation(
            id=session_id,
            application_id=application_id,
            user_id=user_id,
            state=ConversationState.ACTIVE,
            created_at=now,
            updated_at=now,
            expires_at=now + timedelta(minutes=30),
            config=config or ConversationConfig(),
            context=metadata or {}
        )
        
        self.sessions[session_id] = conversation
        if user_id:
            self.user_sessions[user_id].append(session_id)
        
        return conversation
    
    def get_session(self, session_id: str) -> Optional[Conversation]:
        """Retrieve a conversation session"""
        conversation = self.sessions.get(session_id)
        
        if conversation and datetime.utcnow() > conversation.expires_at:
            conversation.state = ConversationState.ENDED
            
        return conversation
    
    def update_session(self, session_id: str, conversation: Conversation):
        """Update session state"""
        conversation.updated_at = datetime.utcnow()
        self.sessions[session_id] = conversation
    
    def end_session(self, session_id: str) -> bool:
        """End a conversation session"""
        if session_id in self.sessions:
            self.sessions[session_id].state = ConversationState.ENDED
            return True
        return False
    
    def get_user_sessions(self, user_id: str) -> List[Conversation]:
        """Get all sessions for a user"""
        session_ids = self.user_sessions.get(user_id, [])
        return [self.sessions[sid] for sid in session_ids if sid in self.sessions]


class ContextStore:
    """Manages conversation context and history"""
    
    def __init__(self):
        self.contexts: Dict[str, Dict[str, Any]] = {}
    
    def store_context(self, conversation_id: str, context: Dict[str, Any]):
        """Store conversation context"""
        self.contexts[conversation_id] = {
            **context,
            "last_updated": datetime.utcnow().isoformat()
        }
    
    def get_context(self, conversation_id: str) -> Dict[str, Any]:
        """Retrieve conversation context"""
        return self.contexts.get(conversation_id, {})
    
    def update_context(self, conversation_id: str, updates: Dict[str, Any]):
        """Update specific context fields"""
        if conversation_id not in self.contexts:
            self.contexts[conversation_id] = {}
        
        self.contexts[conversation_id].update(updates)
        self.contexts[conversation_id]["last_updated"] = datetime.utcnow().isoformat()
    
    def clear_context(self, conversation_id: str):
        """Clear conversation context"""
        if conversation_id in self.contexts:
            del self.contexts[conversation_id]
    
    def manage_context_window(self, messages: List[Message], max_messages: int = 10) -> List[Message]:
        """Keep only recent messages to manage context window"""
        if len(messages) > max_messages:
            # Keep system message if present, plus recent messages
            system_msgs = [m for m in messages if m.role == MessageRole.SYSTEM]
            recent_msgs = messages[-max_messages:]
            return system_msgs + recent_msgs
        return messages


class PromptManager:
    """Manages prompt templates and construction"""
    
    def __init__(self):
        self.templates: Dict[str, PromptTemplate] = {}
        self._init_default_templates()
    
    def _init_default_templates(self):
        """Initialize default prompt templates"""
        default_template = PromptTemplate(
            id="default_assistant",
            name="Default Banking Assistant",
            template="""You are a helpful banking assistant for {bank_name}.
Your role is to help customers with their questions about {services}.

Guidelines:
- Be professional and courteous
- Provide accurate information
- Do not provide investment advice
- Always prioritize customer privacy and security

Customer Context:
- Customer Tier: {customer_tier}
- Available Services: {available_services}
""",
            variables=["bank_name", "services", "customer_tier", "available_services"]
        )
        self.templates["default_assistant"] = default_template
    
    def register_template(self, template: PromptTemplate):
        """Register a new prompt template"""
        self.templates[template.id] = template
    
    def get_template(self, template_id: str) -> Optional[PromptTemplate]:
        """Retrieve a prompt template"""
        return self.templates.get(template_id)
    
    def build_prompt(
        self, 
        template_id: str, 
        variables: Dict[str, Any],
        conversation_history: Optional[List[Message]] = None
    ) -> str:
        """Build a prompt from template with variables"""
        template = self.get_template(template_id)
        if not template:
            raise ValueError(f"Template {template_id} not found")
        
        # Fill in template variables
        prompt = template.template.format(**variables)
        
        # Add conversation history if provided
        if conversation_history:
            history_text = "\n\nConversation History:\n"
            for msg in conversation_history[-5:]:  # Last 5 messages
                history_text += f"{msg.role.value}: {msg.content}\n"
            prompt += history_text
        
        return prompt


class KnowledgeManager:
    """Manages knowledge base and retrieval (simple in-memory implementation)"""
    
    def __init__(self):
        self.documents: Dict[str, KnowledgeDocument] = {}
        self.index: Dict[str, List[str]] = defaultdict(list)  # Simple keyword index
        self._init_sample_knowledge()
    
    def _init_sample_knowledge(self):
        """Initialize with sample banking knowledge"""
        sample_docs = [
            {
                "title": "Savings Account Information",
                "content": "Our savings accounts offer competitive interest rates starting at 2.5% APY. Minimum opening deposit is $100. No monthly fees for balances above $500.",
                "category": "products",
                "application_id": "app_retail_banking"
            },
            {
                "title": "Checking Account Features",
                "content": "Our checking accounts include free ATM withdrawals, mobile check deposit, and overdraft protection. No minimum balance required.",
                "category": "products",
                "application_id": "app_retail_banking"
            },
            {
                "title": "Account Opening Requirements",
                "content": "To open an account, you need: valid government-issued ID, Social Security Number, proof of address, and initial deposit.",
                "category": "procedures",
                "application_id": "app_retail_banking"
            }
        ]
        
        for doc_data in sample_docs:
            self.add_document(**doc_data)
    
    def add_document(
        self, 
        title: str, 
        content: str, 
        category: str,
        application_id: str,
        metadata: Optional[Dict[str, Any]] = None
    ) -> KnowledgeDocument:
        """Add a document to the knowledge base"""
        doc_id = f"doc_{uuid.uuid4().hex[:12]}"
        
        doc = KnowledgeDocument(
            id=doc_id,
            title=title,
            content=content,
            category=category,
            application_id=application_id,
            metadata=metadata or {}
        )
        
        self.documents[doc_id] = doc
        
        # Simple keyword indexing
        keywords = set(title.lower().split() + content.lower().split())
        for keyword in keywords:
            if len(keyword) > 3:  # Index words longer than 3 chars
                self.index[keyword].append(doc_id)
        
        return doc
    
    def search(self, query: str, application_id: str, top_k: int = 5) -> List[KnowledgeDocument]:
        """Simple keyword-based search"""
        query_terms = set(query.lower().split())
        
        # Score documents by keyword matches
        doc_scores: Dict[str, int] = defaultdict(int)
        for term in query_terms:
            if len(term) > 3:
                for doc_id in self.index.get(term, []):
                    doc = self.documents[doc_id]
                    if doc.application_id == application_id:
                        doc_scores[doc_id] += 1
        
        # Sort by score and return top_k
        sorted_docs = sorted(doc_scores.items(), key=lambda x: x[1], reverse=True)[:top_k]
        return [self.documents[doc_id] for doc_id, _ in sorted_docs]
    
    def get_document(self, doc_id: str) -> Optional[KnowledgeDocument]:
        """Retrieve a specific document"""
        return self.documents.get(doc_id)


class ConversationOrchestrator:
    """Main orchestration service that coordinates all components"""
    
    def __init__(
        self,
        session_manager: SessionManager,
        context_store: ContextStore,
        prompt_manager: PromptManager,
        knowledge_manager: KnowledgeManager
    ):
        self.session_manager = session_manager
        self.context_store = context_store
        self.prompt_manager = prompt_manager
        self.knowledge_manager = knowledge_manager
    
    def create_conversation(
        self,
        application_id: str,
        user_id: Optional[str] = None,
        config: Optional[ConversationConfig] = None,
        metadata: Optional[Dict[str, Any]] = None
    ) -> Conversation:
        """Create a new conversation"""
        conversation = self.session_manager.create_session(
            application_id=application_id,
            user_id=user_id,
            config=config,
            metadata=metadata
        )
        
        # Initialize context
        self.context_store.store_context(conversation.id, {
            "application_id": application_id,
            "user_id": user_id,
            "created_at": conversation.created_at.isoformat()
        })
        
        return conversation
    
    def send_message(
        self,
        conversation_id: str,
        content: str,
        metadata: Optional[Dict[str, Any]] = None
    ) -> Message:
        """Process a user message and generate response"""
        # Get conversation
        conversation = self.session_manager.get_session(conversation_id)
        if not conversation:
            raise ValueError(f"Conversation {conversation_id} not found")
        
        if conversation.state != ConversationState.ACTIVE:
            raise ValueError(f"Conversation is {conversation.state}")
        
        # Add user message
        user_message = Message(
            role=MessageRole.USER,
            content=content,
            metadata=metadata or {}
        )
        conversation.messages.append(user_message)
        
        # Perform knowledge retrieval if enabled
        knowledge_context = ""
        if conversation.config.enable_rag:
            docs = self.knowledge_manager.search(
                query=content,
                application_id=conversation.application_id,
                top_k=3
            )
            if docs:
                knowledge_context = "\n\nRelevant Information:\n"
                for i, doc in enumerate(docs, 1):
                    knowledge_context += f"[{i}] {doc.title}: {doc.content}\n"
        
        # Build prompt
        prompt_variables = {
            "bank_name": "Example Bank",
            "services": "checking, savings, loans",
            "customer_tier": conversation.context.get("customer_tier", "standard"),
            "available_services": "account inquiry, transactions, products"
        }
        
        system_prompt = self.prompt_manager.build_prompt(
            template_id="default_assistant",
            variables=prompt_variables,
            conversation_history=conversation.messages[-5:]
        )
        
        # Simulate LLM response (in production, call actual LLM API)
        response_content = self._generate_mock_response(
            user_content=content,
            system_prompt=system_prompt,
            knowledge_context=knowledge_context
        )
        
        # Create assistant message
        assistant_message = Message(
            role=MessageRole.ASSISTANT,
            content=response_content,
            metadata={
                "model": conversation.config.model,
                "temperature": conversation.config.temperature,
                "knowledge_used": bool(knowledge_context)
            }
        )
        conversation.messages.append(assistant_message)
        
        # Manage context window
        conversation.messages = self.context_store.manage_context_window(
            conversation.messages,
            max_messages=20
        )
        
        # Update session
        self.session_manager.update_session(conversation_id, conversation)
        
        # Update context
        self.context_store.update_context(conversation_id, {
            "last_message": content,
            "message_count": len(conversation.messages)
        })
        
        return assistant_message
    
    def _generate_mock_response(
        self,
        user_content: str,
        system_prompt: str,
        knowledge_context: str
    ) -> str:
        """Mock LLM response - replace with actual LLM API call in production"""
        
        # Simple rule-based responses for demo
        content_lower = user_content.lower()
        
        if "balance" in content_lower or "account" in content_lower:
            return "I'd be happy to help you check your account balance. For security purposes, please verify your identity through our secure authentication system."
        
        elif "savings" in content_lower:
            if knowledge_context:
                return f"Based on our current offerings: {knowledge_context}\n\nWould you like to open a savings account or learn more about our features?"
            return "We offer competitive savings accounts with great rates. Would you like to know more details?"
        
        elif "checking" in content_lower:
            if knowledge_context:
                return f"Here's information about our checking accounts: {knowledge_context}\n\nIs there anything specific you'd like to know?"
            return "Our checking accounts come with many convenient features. What would you like to know?"
        
        elif "open" in content_lower and "account" in content_lower:
            if knowledge_context:
                return f"To open an account, here's what you need: {knowledge_context}\n\nI can help guide you through the process. Would you like to proceed?"
            return "I can help you open an account. You'll need some basic information and documents. Shall we get started?"
        
        else:
            return f"Thank you for your message. I'm here to help with your banking needs. Could you please provide more details about what you're looking for?"
    
    def get_conversation_history(
        self,
        conversation_id: str,
        limit: int = 50
    ) -> List[Message]:
        """Get conversation message history"""
        conversation = self.session_manager.get_session(conversation_id)
        if not conversation:
            raise ValueError(f"Conversation {conversation_id} not found")
        
        return conversation.messages[-limit:]
    
    def end_conversation(self, conversation_id: str) -> bool:
        """End a conversation"""
        success = self.session_manager.end_session(conversation_id)
        if success:
            self.context_store.clear_context(conversation_id)
        return success


# ============================================================================
# FastAPI Application
# ============================================================================

app = FastAPI(
    title="AI Conversational Platform",
    description="Foundation implementation with core services",
    version="1.0.0"
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Initialize services
session_manager = SessionManager()
context_store = ContextStore()
prompt_manager = PromptManager()
knowledge_manager = KnowledgeManager()
orchestrator = ConversationOrchestrator(
    session_manager=session_manager,
    context_store=context_store,
    prompt_manager=prompt_manager,
    knowledge_manager=knowledge_manager
)


# ============================================================================
# API Endpoints
# ============================================================================

@app.get("/")
def root():
    """Root endpoint"""
    return {
        "service": "AI Conversational Platform",
        "version": "1.0.0",
        "status": "operational"
    }

@app.get("/health")
def health_check():
    """Health check endpoint"""
    return {
        "status": "healthy",
        "timestamp": datetime.utcnow().isoformat(),
        "services": {
            "session_manager": "operational",
            "context_store": "operational",
            "prompt_manager": "operational",
            "knowledge_manager": "operational",
            "orchestrator": "operational"
        }
    }

@app.post("/v1/conversations", response_model=Conversation)
def create_conversation(request: CreateConversationRequest):
    """Create a new conversation"""
    try:
        conversation = orchestrator.create_conversation(
            application_id=request.application_id,
            user_id=request.user_id,
            config=request.config,
            metadata=request.metadata
        )
        return conversation
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/v1/conversations/{conversation_id}", response_model=Conversation)
def get_conversation(conversation_id: str):
    """Get conversation details"""
    conversation = session_manager.get_session(conversation_id)
    if not conversation:
        raise HTTPException(status_code=404, detail="Conversation not found")
    return conversation

@app.post("/v1/conversations/{conversation_id}/messages", response_model=Message)
def send_message(conversation_id: str, request: SendMessageRequest):
    """Send a message in a conversation"""
    try:
        message = orchestrator.send_message(
            conversation_id=conversation_id,
            content=request.content,
            metadata=request.metadata
        )
        return message
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/v1/conversations/{conversation_id}/messages", response_model=List[Message])
def get_conversation_history(conversation_id: str, limit: int = 50):
    """Get conversation message history"""
    try:
        messages = orchestrator.get_conversation_history(
            conversation_id=conversation_id,
            limit=limit
        )
        return messages
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))

@app.delete("/v1/conversations/{conversation_id}")
def end_conversation(conversation_id: str):
    """End a conversation"""
    success = orchestrator.end_conversation(conversation_id)
    if not success:
        raise HTTPException(status_code=404, detail="Conversation not found")
    return {"status": "ended", "conversation_id": conversation_id}

@app.get("/v1/conversations/{conversation_id}/context")
def get_conversation_context(conversation_id: str):
    """Get conversation context"""
    context = context_store.get_context(conversation_id)
    if not context:
        raise HTTPException(status_code=404, detail="Context not found")
    return context

@app.post("/v1/knowledge/documents", response_model=KnowledgeDocument)
def add_knowledge_document(
    title: str,
    content: str,
    category: str,
    application_id: str,
    metadata: Optional[Dict[str, Any]] = None
):
    """Add a document to the knowledge base"""
    doc = knowledge_manager.add_document(
        title=title,
        content=content,
        category=category,
        application_id=application_id,
        metadata=metadata
    )
    return doc

@app.post("/v1/knowledge/search", response_model=List[KnowledgeDocument])
def search_knowledge(request: SearchRequest):
    """Search the knowledge base"""
    docs = knowledge_manager.search(
        query=request.query,
        application_id=request.application_id,
        top_k=request.top_k
    )
    return docs

@app.get("/v1/prompts/{template_id}", response_model=PromptTemplate)
def get_prompt_template(template_id: str):
    """Get a prompt template"""
    template = prompt_manager.get_template(template_id)
    if not template:
        raise HTTPException(status_code=404, detail="Template not found")
    return template

@app.get("/v1/prompts", response_model=List[PromptTemplate])
def list_prompt_templates():
    """List all prompt templates"""
    return list(prompt_manager.templates.values())


# ============================================================================
# Main entry point
# ============================================================================

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)


-----------

Perfect! I've created a simple but functional foundation implementation of the AI Conversational Platform in Python using FastAPI. Here's what I've built:
Key Components Implemented:
1. Session Manager

Creates and manages conversation sessions
Tracks session lifecycle (ACTIVE, PAUSED, ENDED)
Manages session expiration (30-minute TTL)
Links sessions to users

2. Context Store

Stores conversation context and metadata
Manages context window (keeps recent messages)
Updates context dynamically during conversations

3. Prompt Manager

Template-based prompt system
Variable substitution
Default banking assistant template included
Supports conversation history injection

4. Knowledge Manager

Simple in-memory knowledge base
Keyword-based search (can be upgraded to vector search)
Pre-loaded with sample banking knowledge
Supports RAG (Retrieval Augmented Generation)

5. Conversation Orchestrator

Coordinates all services
Handles message flow
Integrates knowledge retrieval
Mock LLM responses (easily replaceable with real API)

API Endpoints:

POST /v1/conversations - Create conversation
GET /v1/conversations/{id} - Get conversation details
POST /v1/conversations/{id}/messages - Send message
GET /v1/conversations/{id}/messages - Get history
DELETE /v1/conversations/{id} - End conversation
POST /v1/knowledge/search - Search knowledge base
POST /v1/knowledge/documents - Add document
GET /v1/prompts - List prompt templates

To Run:
bash# Install dependencies
pip install fastapi uvicorn pydantic

# Run the server
python app.py

# Server will start at http://localhost:8000
# API docs at http://localhost:8000/docs
Example Usage:
pythonimport requests

# Create conversation
resp = requests.post("http://localhost:8000/v1/conversations", json={
    "application_id": "app_retail_banking",
    "user_id": "user_123",
    "config": {"enable_rag": True}
})
conv_id = resp.json()["id"]

# Send message
resp = requests.post(f"http://localhost:8000/v1/conversations/{conv_id}/messages", json={
    "content": "Tell me about savings accounts"
})
print(resp.json()["content"])
The implementation is production-ready for extension - you can easily replace the mock LLM with real API calls to OpenAI, Anthropic, or other providers!RetryClaude does not have the ability to run the code it generates yet.Scan you create a simple conversational UI with streaming chat using the suggested apisAI Conversational Platform - Chat UIInteractive artifact Perfect! I've created a beautiful, modern chat UI with streaming capabilities. Here's what's included:
Features:
✨ UI/UX

Modern gradient design with smooth animations
Message bubbles with avatars
Real-time typing indicators
Smooth message sliding animations
Auto-scrolling to latest message
Responsive design

🚀 Functionality

Streaming effect - Messages appear word-by-word for natural feel
Session management - Automatically creates conversation on load
Quick action buttons - Pre-set questions for common queries
Knowledge base indicator - Shows when RAG is used
Error handling - Graceful error messages
Keyboard shortcuts - Enter to send, Shift+Enter for new line
Auto-resizing textarea - Expands as you type

🔧 Technical Features

Connects to the FastAPI backend
RESTful API integration
Session tracking with conversation ID
Timestamp display
Metadata tracking
Loading states and disabled inputs during processing

To Use:

Start the FastAPI backend first:

bashpython app.py

Open the HTML file in your browser or serve it:

bash# Simple HTTP server
python -m http.server 8080
# Then open http://localhost:8080

Start chatting! Try the quick action buttons or type your own questions.

Quick Test Messages:

"Tell me about savings accounts"
"What are checking account features?"
"How do I open an account?"
"Check my account balance"

The UI simulates streaming by displaying the response word-by-word, creating a more engaging conversational experience. The backend returns full responses, but the frontend creates the streaming illusion for better UX!