package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// Utility function: random hex string generator
//
// Returns a random 64 byte hex string
func generateChallenge() string {
	buf := make([]byte, 64)
	_, err := rand.Read(buf)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	c := hex.EncodeToString(buf)
	return c
}

// Utility function: sha256 hash content with key
//
// Returns sha256 hash for content using provided secret
func getHash(content []byte, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write(content)
	hash := hex.EncodeToString(h.Sum(nil))
	return hash
}

// Utility function: fetches random advice
//
// Returns random advice from https://api.adviceslip.com
func getRandomAdvice() string {
	resp, _ := http.Get("https://api.adviceslip.com/advice")

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	advice, _ := data["slip"].(map[string]interface{})["advice"].(string)

	defer resp.Body.Close()
	return advice
}
