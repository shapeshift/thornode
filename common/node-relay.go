package common

// This file is designed to sign and broadcast messages from node operators to a discord bot

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"gitlab.com/thorchain/thornode/common/cosmos"
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

func (n *NodeRelay) fetchUUid() error {
	// GET UUID PREFIX
	resp, err := http.Get("https://node-relay-bot.herokuapp.com/uuid_prefix")
	if err != nil {
		return err
	}
	//We Read the response body on the line below.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	//Convert the body to type string
	prefix := string(body)

	// GENERATE RANDOM UUID, with PREFIX. This is to defense against replay attacks
	id := uuid.New().String()
	parts := strings.Split(id, "-")
	parts[0] = prefix
	n.Msg.UUID = strings.Join(parts, "-")
	return nil
}

func (n *NodeRelay) sign() error {
	kbs, err := cosmos.GetKeybase(os.Getenv("CHAIN_HOME_FOLDER"))
	if err != nil {
		return err
	}

	buf := []byte(fmt.Sprintf("%s|%s|%s", n.Msg.UUID, n.Msg.Channel, n.Msg.Text))
	sig, _, err := kbs.Keybase.Sign(kbs.SignerName, buf)
	if err != nil {
		return err
	}

	info, err := kbs.Keybase.Key(kbs.SignerName)
	if err != nil {
		return err
	}
	n.PubKey = base64.StdEncoding.EncodeToString(info.GetPubKey().Bytes())
	n.Signature = base64.StdEncoding.EncodeToString(sig)

	return nil
}

func (n *NodeRelay) Prepare() error {
	if err := n.fetchUUid(); err != nil {
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
	//Leverage Go's HTTP Post function to make request
	resp, err := http.Post("https://node-relay-bot.herokuapp.com/msg", "application/json", responseBody)
	//Handle Error
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	sb := string(body)

	return sb, nil
}
