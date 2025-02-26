package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// RequestBody defines the JSON structure for incoming requests.
type RequestBody struct {
	Location string `json:"location"` // e.g., "San Francisco, CA"
	Query    string `json:"query"`    // additional preferences (optional)
}

// Restaurant represents a simple restaurant object.
type Restaurant struct {
	Name     string   `json:"name"`
	Address  string   `json:"address"`
	Price    float64  `json:"price"`
	Rating   float64  `json:"rating"`
	Distance float64  `json:"distance"`
	Reviews  []string `json:"reviews"`
}

// ChatMessage represents a single chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest defines the payload sent to the Ollama chat endpoint.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// ChatResponse defines the expected response from the Ollama chat endpoint.
type ChatResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// getRestaurants simulates fetching restaurant data for a given location.
// Replace this stub with real API calls (e.g., Yelp, Google Places) as needed.
func getRestaurants(location string) ([]Restaurant, error) {
	restaurants := []Restaurant{
		{"The Gourmet Spot", "123 Main St", 25.0, 4.5, 0.5, []string{"Great food!", "Excellent service!"}},
		{"Budget Bites", "456 Elm St", 15.0, 4.0, 0.8, []string{"Affordable and tasty.", "Good value!"}},
		{"Fancy Eats", "789 Oak St", 40.0, 4.7, 1.2, []string{"High-end experience.", "Loved the ambiance!"}},
	}
	return restaurants, nil
}

// callOllama constructs a chat request and sends it to the Ollama /api/chat endpoint.
// It extracts and returns the assistant's message content.
func callOllama(prompt string) (string, error) {
	// Use a default model (or set via OLLAMA_MODEL environment variable)
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "llama3.2"
	}

	chatReq := ChatRequest{
		Model: model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: false,
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	// Use OLLAMA_URL environment variable if set, otherwise default to localhost.
	baseURL := os.Getenv("OLLAMA_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	chatEndpoint := baseURL + "/api/chat"

	resp, err := http.Post(chatEndpoint, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("HTTP POST to Ollama failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Ollama response body: %w", err)
	}

	log.Printf("Ollama raw response: %s", string(body))

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal Ollama response: %w", err)
	}

	return chatResp.Message.Content, nil
}

// handleRequest processes the incoming HTTP request, builds a restaurant summary prompt,
// calls the Ollama backend for a tailored recommendation, and returns an OpenAI-compatible response.
func handleRequest(w http.ResponseWriter, r *http.Request) {
	var reqData RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	restaurants, err := getRestaurants(reqData.Location)
	if err != nil {
		http.Error(w, "Error fetching restaurant data", http.StatusInternalServerError)
		return
	}

	// Build the prompt by incorporating the location, query, and restaurant details.
	prompt := fmt.Sprintf("User is looking for restaurants near %s", reqData.Location)
	if reqData.Query != "" {
		prompt += fmt.Sprintf(" with query '%s'.", reqData.Query)
	} else {
		prompt += "."
	}
	prompt += "\nHere are some options:\n"
	for _, r := range restaurants {
		prompt += fmt.Sprintf("- %s at %s, Price: $%.2f, Rating: %.1f, Distance: %.1f miles. Reviews: %v\n",
			r.Name, r.Address, r.Price, r.Rating, r.Distance, r.Reviews)
	}
	prompt += "\nPlease provide a friendly recommendation based on the above options."

	aiOutput, err := callOllama(prompt)
	if err != nil {
		log.Printf("callOllama error: %v", err)
		http.Error(w, "Error generating AI response", http.StatusInternalServerError)
		return
	}

	// Format the final response to mimic OpenAI's chat completion format.
	response := map[string]interface{}{
		"id":      "chatcmpl-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"message":       map[string]string{"role": "assistant", "content": aiOutput},
				"finish_reason": "stop",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/v1/chat/completions", handleRequest)
	port := "8080"
	log.Printf("Server is running on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
