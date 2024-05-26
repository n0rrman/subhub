package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// Holds the storage container for the subscription data
//
// Requires the following functions:
//   - type hubStore struct {}
//   - func (h *hubStore) init()
//   - func (h *hubStore) addSubscriber(callback string, secret string, topic string)
//   - func (h *hubStore) removeSubscriber(callback string, topic string)
//   - func (h *hubStore) getAllSubsByTopic(callback string) []subscriber
type hub struct {
	store hubStore
}

// A single subscriber instance
//
// subscriber	callback URL to the subscriber
// secret		HMAC key provided on subscription
// topic		topic subscribed to
type subscription struct {
	subscriber string
	secret     string
	topic      string
}

// Serves 404 Not Found routes
//
// Returns 404 Not found and a simple Not Found HTML page
func (*hub) invalidRoute(ctx echo.Context) error {
	return ctx.HTML(http.StatusNotFound, `
		<div style="padding: 0.25rem;">
			Invalid path! Send a 
			<code style="background-color: rgb(226 232 240); border-radius: 0.125rem; padding: 0.2rem">POST</code> 
			request to 
			<code style="background-color: rgb(226 232 240); border-radius: 0.125rem; padding: 0.2rem;">/</code> 
			with the header parameter 
			<code style="background-color: rgb(226 232 240); border-radius: 0.125rem; padding: 0.2rem">hub.topic</code> 
			set to 
			<code style="background-color: rgb(226 232 240); border-radius: 0.125rem; padding: 0.2rem">advice</code> 
			to subscribe to the hub, or a 
			<code style="background-color: rgb(226 232 240); border-radius: 0.125rem; padding: 0.2rem">GET</code> 
			request to 
			<code style="background-color: rgb(226 232 240); border-radius: 0.125rem; padding: 0.2rem;">/publish</code> 
			to broadcast a friendly advice to every subscriber.
		</div>`)
}

// Serves new subscription requests
//
// Checks and verifies WebSub params for new subscribe and unsubscribe requests. If accepted, the new status is saved in the hubStore.
// Returns 202 Accepted on valid requests and 4xx / 5xx on failed requests.
func (h *hub) handleSubscriber(ctx echo.Context) error {
	// REQUIRED WebSub params
	// "MUST accept a subscription request with the parameters hub.callback, hub.mode and hub.topic."
	callback := ctx.FormValue("hub.callback")
	mode := ctx.FormValue("hub.mode")
	topic := ctx.FormValue("hub.topic")

	// OPTIONAL WebSub params
	// "MUST accept a subscription request with a hub.secret parameter."
	secret := ctx.FormValue("hub.secret")
	timestamp := time.Now().Unix()

	// Check required params, return 400 StatusBadRequest if any required param is missing
	if (callback == "") || (mode == "") || (topic == "") {
		// Send denial GET request to callback URL in a new goroutine
		go sendGET(callback, "denied", topic, "")
		return ctx.String(http.StatusBadRequest, "Error: params hub.callback, hub.mode, and hub.topic required!")
	}

	// Verify request intent in new goroutine and return 202 Accepted HTTP status code
	go h.verifyIntent(callback, secret, mode, topic, timestamp)
	return ctx.String(http.StatusAccepted, "Request accepted!")
}

// Helper function to send HTTP GET request
//
// Sends a HTTP GET request to the callback with provided query params.
// Returns the body of the response as a []byte, and the status code.
func sendGET(callback string, mode string, topic string, challenge string) ([]byte, int) {
	// Create a new request
	req, _ := http.NewRequest("GET", callback, nil)

	// Attach query params
	q := req.URL.Query()
	q.Add("hub.mode", mode)
	q.Add("hub.topic", topic)
	if challenge != "" {
		q.Add("hub.challenge", challenge)
	}
	req.URL.RawQuery = q.Encode()

	// Create a client and send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0
	}

	// Convert body to []byte and close the response body
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	return body, resp.StatusCode
}

// Verify intent with a GET request to callback URL
//
// Adds subscription to the hubStore if verification is successful.
// Returns early if verification fails.
func (h *hub) verifyIntent(callback string, secret string, mode string, topic string, timestamp int64) {
	// Generate new challenge
	c := generateChallenge()

	// Send GET request
	body, statusCode := sendGET(callback, mode, topic, c)

	// Challenge not echoed or status code 404 -> verification failed, return from function
	if c != string(body[:]) || statusCode == http.StatusNotFound {
		return
	}

	// Mode: unsubscribe -> remove subscriber
	// "MUST support unsubscription requests."
	if mode == "unsubscribe" {
		h.store.removeSubscriber(callback, topic)
		fmt.Println(callback, "unsubscribed from", topic)
	}

	// Verification passed
	// Mode: subscribe -> add new subscriber
	if mode == "subscribe" {
		h.store.addSubscriber(callback, secret, topic, timestamp)
		fmt.Println(callback, "(re)subscribde to", topic)
	}
}

// A dummy publisher for testing/demo purpose
//
// Broadcasts a friendly advice to all subscribers of the topic "advice"
func (h *hub) dummyPublisher(ctx echo.Context) error {

	// Get all subscriptions to the topic "advice"
	topic := "advice"
	subscribers := h.store.getAllSubsByTopic(topic)

	// Fetch friendly advice
	advice := getRandomAdvice()

	// Send content to subscribers in separate goroutines
	for _, s := range subscribers {
		go func(sub subscription, a string) {
			success := sendContent(sub.subscriber, sub.secret, sub.topic, a)
			// If not succesful -> remove subscriber
			if !success {
				h.store.removeSubscriber(sub.subscriber, sub.topic)
			}
		}(s, advice)

	}

	return ctx.String(http.StatusOK, "\""+advice+"\" was sent to all subscribers of the topic \"advice\".")
}

// Send published content to subscribers
//
// Packs content in JSON and generates HMAC signature for header.
// Sends content in a POST request to callback URL.
// Returns true if successful, otherwise false.
func sendContent(url string, secret string, topic string, content string) bool {
	// Convert content to []byte for hashing and JSON convertions
	jsonBody := []byte("{\"topic\":\"" + topic + "\", \"content\":\"" + content + "\"}")

	// Generate HMAC hash for header
	h := getHash(jsonBody, secret)

	// Init POST request with JSON body to callback URL
	// "MUST send content distribution requests with a matching content type of the topic URL." -> JSON predefined
	req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))

	// Add required headers
	// "MUST send a X-Hub-Signature header if the subscription was made with a hub.secret"
	req.Header.Add("X-Hub-Signature", "sha256="+h)
	req.Header.Add("Content-Type", "application/json")

	// Open client and send POST request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Returns true of succesful, otherwise false
	return resp.StatusCode == 200
}
