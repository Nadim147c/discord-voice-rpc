package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"

	"github.com/go-viper/mapstructure/v2"
	"github.com/gorilla/websocket"
)

const ClientID = "207646673902501888"

func must[T any](v T, e error) T {
	if e != nil {
		panic(e)
	}
	return v
}

type Client struct {
	ws               *websocket.Conn
	authcode         string
	loggedIn         bool
	currentChannelID string
	channel          *VoiceChannel
}

func NewClient() *Client {
	c := new(Client)
	return c
}

func (c *Client) Listen(ctx context.Context) error {
	headers := http.Header{}
	headers.Add("Origin", "http://localhost:3000")

	query := url.Values{}
	query.Set("client_id", ClientID)
	query.Set("v", "1")
	url := "ws://127.0.0.1:6463/?" + query.Encode()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, headers)
	if err != nil {
		return err
	}
	defer conn.Close()

	c.ws = conn

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var msg Request
		json.Unmarshal(message, &msg)
		slog.Info("Message", "command", msg.Command, "event", msg.Event)

		if err := c.handleCommand(msg); err != nil {
			slog.Error("failed handle command", "error", err)
		}
	}
}

func (c *Client) handleCommand(msg Request) error {
	switch msg.Command {
	case CmdDispatch:
		return c.handleEvent(msg)
	case CmdAuthorize:
		code, err := msg.Data.GetStringE("code")
		if err != nil {
			return err
		}
		return c.fetchAuthcode(code)
	case CmdAuthenticate:
		if msg.Event == EvtError {
			os.Remove(c.getTokenFile())
			c.authcode = ""
			return c.requestAuthcode()
		}
		c.loggedIn = true
		if err := c.getCurrentVoiceChannel(); err != nil {
			return err
		}
		return c.subscribe(EvtVoiceChannelSelect, nil)
	case CmdGetSelectedVoiceChannel:
		var state VoiceChannel
		err := mapstructure.Decode(msg.Data, &state)
		if err != nil {
			return err
		}
		c.channel = &state
		return c.subscribeVoice(state.ID)
	case CmdSubscribe:
		slog.Info("subscribed to event", "event", msg.Data.GetString("evt"))
		return nil
	default:
		os.WriteFile("unknown-command.json", must(json.Marshal(msg)), 0o640)
		panic("unknown-command " + msg.Command.String())
	}
}

func (c *Client) handleEvent(msg Request) error {
	switch msg.Event {
	case EvtReady:
		if c.authcode != "" {
			return c.authorize(c.authcode)
		}
		return c.requestAuthcode()
	case EvtVoiceStateUpdate, EvtVoiceStateCreate:
		var member VoiceMember
		err := mapstructure.Decode(msg.Data, &member)
		if err != nil {
			return err
		}
		c.setMember(member)
		return c.update()
	case EvtSpeakingStart, EvtSpeakingStop:
		id, err := msg.Data.GetStringE("user_id")
		if err != nil {
			return err
		}
		c.setMemberTalking(id, msg.Event == EvtSpeakingStart)
		return c.update()
	case EvtVoiceChannelSelect:
		if msg.Data.Has("channel_id") {
			c.unsubVoice(c.currentChannelID)
			c.channel = nil
			c.update()
		}
		return c.getCurrentVoiceChannel()
	default:
		os.WriteFile("unknown-event.json", must(json.Marshal(msg)), 0o640)
		panic("unknown-event " + msg.Event.String())
	}
}

// TODO: implemete update
func (c *Client) update() error {
	return json.NewEncoder(os.Stdout).Encode(c.channel)
}

func (c *Client) ensureMembers() {
	if c.channel.Members == nil {
		c.channel.Members = make(VoiceMembers)
	}
}

func (c *Client) setMember(m VoiceMember) {
	if c.channel == nil {
		return
	}
	c.ensureMembers()
	c.channel.Members[m.User.ID] = m
}

func (c *Client) setMemberTalking(id string, state bool) {
	if c.channel == nil {
		return
	}
	c.ensureMembers()
	if m, ok := c.channel.Members[id]; ok {
		m.Talking = state
		c.channel.Members[id] = m
	}
}

func (c *Client) subscribe(event Event, args Map) error {
	req := NewRequest(CmdSubscribe)
	req.Event = event
	req.Args = args
	return c.ws.WriteJSON(req)
}

func (c *Client) unsubscribe(event Event, args map[string]any) error {
	req := NewRequest(CmdUnsubscribe)
	req.Event = event
	req.Args = args
	return c.ws.WriteJSON(req)
}

func (c *Client) subscribeChannel(event Event, channel string) error {
	var m Map
	m.Set("channel_id", channel)
	return c.subscribe(event, m)
}

func (c *Client) unsubscribeChannel(event Event, channel string) error {
	var m Map
	m.Set("channel_id", channel)
	return c.unsubscribe(event, m)
}

var voiceEvents = []Event{
	EvtVoiceStateCreate,
	EvtVoiceStateUpdate,
	EvtVoiceStateDelete,
	EvtSpeakingStart,
	EvtSpeakingStop,
}

func (c *Client) subscribeVoice(channelID string) error {
	if c.currentChannelID != "" {
		if err := c.unsubVoice(channelID); err != nil {
			return err
		}
	}
	c.currentChannelID = channelID
	for event := range slices.Values(voiceEvents) {
		if err := c.subscribeChannel(event, channelID); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) unsubVoice(channelID string) error {
	c.currentChannelID = ""
	for event := range slices.Values(voiceEvents) {
		if err := c.unsubscribeChannel(event, channelID); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) getCurrentVoiceChannel() error {
	req := NewRequest(CmdGetSelectedVoiceChannel)
	return c.ws.WriteJSON(req)
}

func (c *Client) authorize(access string) error {
	req := NewRequest(CmdAuthenticate)
	req.SetArg("access_token", access)
	slog.Info("Requesting authorization via access_token", "access_token", access)
	return c.ws.WriteJSON(req)
}

func (c *Client) requestAuthcode() error {
	req := NewRequest(CmdAuthorize)
	req.Args = map[string]any{
		"client_id": ClientID,
		"scopes": []string{
			"rpc",
			"messages.read",
			"rpc.notifications.read",
			"identify",
		},
		"prompt": "none",
	}

	return c.ws.WriteJSON(req)
}

func (c *Client) fetchAuthcode(code string) error {
	slog.Info("fetching access_token from streamkit api")
	payload := struct {
		Code string `json:"code"`
	}{Code: code}

	buf := bytes.NewBuffer(nil)
	json.NewEncoder(buf).Encode(payload)

	resp, err := http.Post("https://streamkit.discord.com/overlay/token", "application/json", buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get streamkit access_token")
	}

	var streamkitResp struct {
		AccessToken string `json:"access_token"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&streamkitResp); err != nil {
		return err
	}

	c.authcode = streamkitResp.AccessToken

	if err := os.WriteFile(c.getTokenFile(), []byte(c.authcode), 0o640); err != nil {
		slog.Error("failed to cache access_token", "error", err)
	}

	return c.authorize(c.authcode)
}

func (c *Client) getTokenFile() string {
	config := must(os.UserConfigDir())
	return filepath.Join(config, "dcat"+ClientID+".bin") // .bin ext make sense!
}
