package common

// This file is designed to sign and broadcast messages from node operators to a discord bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type NodeRelayMsg struct {
	Text    string `json:"text"`
	Channel string `json:"channel"`
	UUID    string `json:"uuid"`
}

type NodeRelay struct {
	Msg       NodeRelayMsg `json:"msg"`
	Signature string       `json:"signature"`
	PubKey    string       `json:"pubkey"`
}

func NewNodeRelay(channel, text string) *NodeRelay {
	return &NodeRelay{
		Msg: NodeRelayMsg{
			Text:    text,
			Channel: channel,
		},
	}
}

func (n *NodeRelay) fetchUUID() error {
	// GET UUID PREFIX
	resp, err := http.Get("https://node-relay-bot.herokuapp.com/uuid_prefix")
	if err != nil {
		return err
	}
	// We Read the response body on the line below.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// Convert the body to type string
	prefix := string(body)

	// GENERATE RANDOM UUID, with PREFIX. This is to defense against replay attacks
	id := uuid.New().String()
	parts := strings.Split(id, "-")
	parts[0] = prefix
	n.Msg.UUID = strings.Join(parts, "-")
	return nil
}

func (n *NodeRelay) sign() error {
	msg := fmt.Sprintf("%s|%s|%s", n.Msg.UUID, n.Msg.Channel, n.Msg.Text)

	sig, pubkey, err := SignBase64([]byte(msg))
	if err != nil {
		return err
	}

	n.PubKey = pubkey
	n.Signature = sig

	return nil
}

func (n *NodeRelay) Prepare() error {
	if err := n.fetchUUID(); err != nil {
		return err
	}
	if err := n.sign(); err != nil {
		return err
	}
	return nil
}

func (n *NodeRelay) Broadcast() (string, error) {
	postBody, _ := json.Marshal(n)

	// POST to discord bot
	responseBody := bytes.NewBuffer(postBody)
	// Leverage Go's HTTP Post function to make request
	resp, err := http.Post("https://node-relay-bot.herokuapp.com/msg", "application/json", responseBody)
	// Handle Error
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	sb := string(body)

	return sb, nil
}
