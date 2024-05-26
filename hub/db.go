package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Implementation of the hubStore using postgres to store the subscriptions
//
// Holds a pointer to the postgres connection pool
type hubStore struct {
	store *pgxpool.Pool
}

// Initiate the database instance
//
// Connects to the Postgres server, and adds the subscription table.
// Prints error and exits with error code 1 on failure.
func (h *hubStore) init() {
	// Connect to the postgres server
	fmt.Println("Connecting to database...")
	pgURL := "postgres://postgres:password@database:5432/hub"
	var err error
	h.store, err = pgxpool.Connect(context.Background(), pgURL)
	if err != nil {
		h.store.Close()
		fmt.Println(err)
		os.Exit(1)
	}

	// Query to create subscription table
	_, err = h.store.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS subscription (
			id SERIAL PRIMARY KEY,
			subscriber VARCHAR(128) NOT NULL,
			secret VARCHAR(128) NOT NULL,
			topic VARCHAR(128) NOT NULL,
			timestamp BIGINT,
			UNIQUE (subscriber, topic)
		)`)
	if err != nil {
		h.store.Close()
		fmt.Println(err)
		os.Exit(1)
	}

	defer fmt.Println("Database ready!")
}

// Add new subscriber to the database
//
// "MUST allow subscribers to re-request already active subscriptions."
func (h *hubStore) addSubscriber(callback string, secret string, topic string, timestamp int64) {
	// Query to add new subscriber. Ignores old/delayed subscriptions and updates timestamp on resubscriptions.
	_, err := h.store.Exec(context.Background(), `
		INSERT INTO subscription
			(subscriber, secret, topic, timestamp) 
		VALUES 
			($1, $2, $3, $4)
		ON CONFLICT (subscriber, topic) DO UPDATE SET 
		timestamp = CASE
			WHEN EXCLUDED.timestamp > subscription.timestamp
			THEN EXCLUDED.timestamp
			ELSE subscription.timestamp
		END,
		topic = CASE
			WHEN EXCLUDED.timestamp > subscription.timestamp
			THEN EXCLUDED.topic
			ELSE subscription.topic
		END;
	`, callback, secret, topic, timestamp)
	if err != nil {
		h.store.Close()
		fmt.Println(err)
		os.Exit(1)
	}
}

// Remove subscription from the database
//
// Remove a subscription to a specific topic. A user with multiple
// subscriptions keep their other topic subscriptions.
func (h *hubStore) removeSubscriber(callback string, topic string) {
	// Query to remove a subscription
	_, err := h.store.Exec(context.Background(), `
		DELETE FROM subscription
		WHERE subscriber=$1 AND topic=$2
	`, callback, topic)
	if err != nil {
		h.store.Close()
		fmt.Println(err)
		os.Exit(1)
	}
}

// Get all subscriptions to a specific topic.
//
// Return all topic subscriptions in the database
func (h *hubStore) getAllSubsByTopic(topic string) []subscription {
	// Query to fetch all subscriptions
	rows, err := h.store.Query(context.Background(), `
		SELECT subscriber, secret, topic 
		FROM subscription 
		WHERE topic=$1
	`, topic)
	if err != nil {
		h.store.Close()
		fmt.Println(err)
		os.Exit(1)
	}

	// Store the subscriptions in slice
	subscriptions := []subscription{}
	for rows.Next() {
		s := subscription{}
		rows.Scan(&s.subscriber, &s.secret, &s.topic)
		subscriptions = append(subscriptions, s)
	}

	defer rows.Close()
	return subscriptions
}
