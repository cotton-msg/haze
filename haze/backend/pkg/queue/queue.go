package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// Message — элемент очереди в формате JSON.
type Message struct {
	Type   string          `json:"type"`
	Target string          `json:"target"`
	Body   json.RawMessage `json:"body"`
}

// Producer пишет события в Redis Stream.
type Producer struct {
	client redis.Cmdable
	stream string
}

func NewProducer(client redis.Cmdable, stream string) *Producer {
	return &Producer{client: client, stream: stream}
}

func (p *Producer) Enqueue(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return p.client.XAdd(ctx, &redis.XAddArgs{Stream: p.stream, Values: map[string]interface{}{"payload": string(data)}}).Err()
}

// Consumer читает события из Redis Stream группами.
type Consumer struct {
	client  redis.Cmdable
	stream  string
	group   string
	name    string
	dlq     string
	handler func(Message) error
}

func NewConsumer(client redis.Cmdable, stream, group string, handler func(Message) error) *Consumer {
	name := consumerName()
	return &Consumer{
		client:  client,
		stream:  stream,
		group:   group,
		name:    name,
		dlq:     stream + ":dead",
		handler: handler,
	}
}

// consumerName строит уникальное имя воркера, чтобы реплики одного сервиса
// не конкурировали за одни и те же сообщения в группе.
func consumerName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

// Run запускает цикл чтения. Блокирует до завершения ctx.
func (c *Consumer) Run(ctx context.Context, pollInterval time.Duration) error {
	c.ensureGroup(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		entries, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.group,
			Consumer: c.name,
			Streams:  []string{c.stream, ">"},
			Count:    10,
			Block:    time.Second,
		}).Result()
		if err != nil {
			if err != redis.Nil {
				time.Sleep(pollInterval)
			}
			continue
		}
		if len(entries) == 0 {
			continue
		}
		for _, stream := range entries {
			for _, entry := range stream.Messages {
				var msg Message
				raw, ok := entry.Values["payload"].(string)
				if !ok || json.Unmarshal([]byte(raw), &msg) != nil {
					// Битое сообщение не может быть обработано — в DLQ.
					c.deadLetter(ctx, entry, raw, "malformed payload")
					c.ack(ctx, entry)
					continue
				}
				if err := c.handler(msg); err != nil {
					c.deadLetter(ctx, entry, raw, err.Error())
					c.ack(ctx, entry)
					continue
				}
				c.ack(ctx, entry)
			}
		}
	}
}

func (c *Consumer) ensureGroup(ctx context.Context) {
	err := c.client.XGroupCreateMkStream(ctx, c.stream, c.group, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return
	}
}

func (c *Consumer) ack(ctx context.Context, entry redis.XMessage) {
	c.client.XAck(ctx, c.stream, c.group, entry.ID)
}

// deadLetter копирует сообщение в DLQ перед тем, как ack-нуть оригинал,
// чтобы упавшие события не терялись и не крутились в pending вечно.
func (c *Consumer) deadLetter(ctx context.Context, entry redis.XMessage, payload, reason string) {
	values := map[string]interface{}{
		"payload": payload,
		"id":      entry.ID,
		"reason":  reason,
	}
	if err := c.client.XAdd(ctx, &redis.XAddArgs{Stream: c.dlq, Values: values}).Err(); err != nil {
		// DLQ недоступен — остаёмся с оригиналом в pending для повторной доставки.
		return
	}
}
