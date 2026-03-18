// Package main provides a client for the Discord Voice RPC. It listens to the
// Discord Voice Overlay API, retrieves the voice channel state, and prints it
// to stdout as JSON.
//
// This is intended to be used for displaying a Discord overlay on any
// application, specifically for tools like Quickshell.
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

// ClientID is the client id for the discord voice rpc.
//
// NOTE: Change this if you want!
const ClientID = "207646673902501888"

// Client is the discord ipc listener and voice state manager.
type Client struct {
	// ws is the websocket connection to discord.
	ws *websocket.Conn
	// authcode is the authorization code for discord.
	authcode string
	// loggedIn is true if the client is logged in.
	loggedIn bool
	// channelID is the ID of the voice channel.
	channelID string
	// state is the voice channel state.
	state *VoiceState
}

// NewClient creates a new client.
func NewClient() *Client { return new(Client) }

// Listen listens to the discord ipc and handles the commands. It blocks until
// ctx is done.
func (c *Client) Listen(ctx context.Context) error {
	headers := http.Header{}
	headers.Add("Origin", "http://localhost:3000")

	url := should(url.Parse("ws://127.0.0.1:6463/"))
	query := url.Query()
	query.Set("client_id", ClientID)
	query.Set("v", "1")
	url.RawQuery = query.Encode()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url.String(), headers)
	if err != nil {
		return err
	}
	defer conn.Close()

	c.ws = conn

	authcode, err := os.ReadFile(c.getTokenFile())
	if err == nil {
		c.authcode = string(bytes.TrimSpace(authcode))
	}

	// force close the connection when context is done.
	context.AfterFunc(ctx, func() { conn.Close() })

	for {
		_, message, err := conn.ReadMessage()

		// we ignore the connection err when context is done.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		var msg Message
		json.Unmarshal(message, &msg)
		slog.Info("Message", "command", msg.Command, "event", msg.Event)

		if err := c.handleCommand(msg); err != nil {
			slog.Error("failed handle command", "error", err)
		}
	}
}

// handleCommand handles the commands from discord.
func (c *Client) handleCommand(msg Message) error {
	switch msg.Command {
	case CmdDispatch:
		return c.handleEvent(msg)
	case CmdAuthorize:
		code, err := msg.Data.GetStringE("code")
		if err != nil {
			return err
		}
		slog.Debug("Received authcode from discord client", "code", code)
		return c.fetchAccessToken(code)
	case CmdAuthenticate:
		if msg.Event == EvtError {
			slog.Error("Authentication failed", "msg", msg.Data)
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
		if msg.Data == nil {
			c.reset()
			return nil
		}

		var state VoiceState
		err := mapstructure.Decode(msg.Data, &state)
		if err != nil {
			return err
		}
		c.state = &state
		return c.subscribeVoice(state.ID)
	case CmdSubscribe:
		slog.Info("subscribed to event", "event", msg.Data.GetString("evt"))
		return nil
	case CmdUnsubscribe:
		slog.Info("unsubscribed from event", "event", msg.Data.GetString("evt"))
		return nil
	default:
		// TODO: remove this!
		os.WriteFile("unknown-command.json", should(json.Marshal(msg)), 0o640)
		panic("unknown-command " + msg.Command.String())
	}
}

// handleEvent handles the command from discord which is an event.
func (c *Client) handleEvent(msg Message) error {
	// always print the output when new event is received.
	defer c.update()

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
		return nil
	case EvtVoiceStateDelete:
		var member VoiceMember
		err := mapstructure.Decode(msg.Data, &member)
		if err != nil {
			return err
		}
		c.deleteMember(member.User.ID)
		return nil
	case EvtSpeakingStart, EvtSpeakingStop:
		id, err := msg.Data.GetStringE("user_id")
		if err != nil {
			return err
		}
		c.setMemberTalking(id, msg.Event == EvtSpeakingStart)
		return nil
	case EvtVoiceChannelSelect:
		if msg.Data.GetString("channel_id") == "" {
			c.reset()
			return nil
		}
		slog.Info("Voice channel detected", "id", msg.Data.GetString("channel_id"))
		c.unsubVoice(c.channelID)
		c.state = nil
		return c.getCurrentVoiceChannel()
	default:
		// TODO: remove this!
		os.WriteFile("unknown-event.json", should(json.Marshal(msg)), 0o640)
		panic("unknown-event " + msg.Event.String())
	}
}

// update prints the output of c.state to stdout.
func (c *Client) update() error {
	return json.NewEncoder(os.Stdout).Encode(c.state)
}

// reset resets the client state. Generate when user leaves voice channel.
func (c *Client) reset() {
	c.state = nil
	c.channelID = ""
}

// ensureMembers ensures that the c.state.Members is initialized.
func (c *Client) ensureMembers() {
	if c.state.Members == nil {
		c.state.Members = make(VoiceMembers)
	}
}

// deleteMember deletes the member from the client state.
func (c *Client) deleteMember(id string) {
	if c.state == nil {
		return
	}
	c.ensureMembers()
	delete(c.state.Members, id)
}

// setMember sets the member in the client state.
func (c *Client) setMember(m VoiceMember) {
	if c.state == nil {
		return
	}
	c.ensureMembers()
	c.state.Members[m.User.ID] = m
}

// setMemberTalking sets the talking state of the member in the client state.
func (c *Client) setMemberTalking(id string, state bool) {
	if c.state == nil {
		return
	}
	c.ensureMembers()
	m, ok := c.state.Members[id]
	if !ok {
		// user is missing update the vc to get thte user
		c.getCurrentVoiceChannel()
		return
	}
	m.Talking = state
	c.state.Members[id] = m
}

// subscribe subscribes to an event.
func (c *Client) subscribe(event Event, args Map) error {
	req := NewMessage(CmdSubscribe)
	req.Event = event
	req.Args = args
	return c.ws.WriteJSON(req)
}

// unsubscribe unsubscribes from an event.
func (c *Client) unsubscribe(event Event, args map[string]any) error {
	req := NewMessage(CmdUnsubscribe)
	req.Event = event
	req.Args = args
	return c.ws.WriteJSON(req)
}

// subscribeChannel subscribes to an event on a specific channel.
func (c *Client) subscribeChannel(event Event, channel string) error {
	var m Map
	m.Set("channel_id", channel)
	return c.subscribe(event, m)
}

// unsubscribeChannel unsubscribes from an event on a specific channel.
func (c *Client) unsubscribeChannel(event Event, channel string) error {
	var m Map
	m.Set("channel_id", channel)
	return c.unsubscribe(event, m)
}

// voiceEvents is a list of events that we create about.
var voiceEvents = []Event{
	EvtVoiceStateCreate,
	EvtVoiceStateUpdate,
	EvtVoiceStateDelete,
	EvtSpeakingStart,
	EvtSpeakingStop,
}

// subscribeVoice subscribes to all voice events for a specific channel.
// Returns after first subscription error.
func (c *Client) subscribeVoice(channelID string) error {
	if c.channelID != "" {
		if err := c.unsubVoice(channelID); err != nil {
			return err
		}
	}
	c.channelID = channelID
	for event := range slices.Values(voiceEvents) {
		if err := c.subscribeChannel(event, channelID); err != nil {
			return err
		}
	}
	return nil
}

// unsubVoice unsubscribes from all voice events for a specific channel.
// Returns after first unsubscription error.
func (c *Client) unsubVoice(channelID string) error {
	c.channelID = ""
	for event := range slices.Values(voiceEvents) {
		if err := c.unsubscribeChannel(event, channelID); err != nil {
			return err
		}
	}
	return nil
}

// getCurrentVoiceChannel gets the state of current voice channel. Run this when
// you want update and any user is missing from cached state of current voice
// channel.
func (c *Client) getCurrentVoiceChannel() error {
	req := NewMessage(CmdGetSelectedVoiceChannel)
	return c.ws.WriteJSON(req)
}

// authorize authorizes the client with an access token.
func (c *Client) authorize(access string) error {
	req := NewMessage(CmdAuthenticate)
	req.SetArg("access_token", access)
	slog.Info("Requesting authorization via access_token")
	return c.ws.WriteJSON(req)
}

// requestAuthcode requests an authcode from Discord Client.
func (c *Client) requestAuthcode() error {
	req := NewMessage(CmdAuthorize)
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
	slog.Info("Requesting authcode from discord client")
	return c.ws.WriteJSON(req)
}

type StreamkitRequest struct {
	Code string `json:"code"`
}

// fetchAccessToken fetches an access token from Discord streamkit overlay api.
func (c *Client) fetchAccessToken(code string) error {
	slog.Info("fetching access_token from streamkit api")
	payload := StreamkitRequest{Code: code}
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
	slog.Debug("Received access token", "access_token", streamkitResp.AccessToken)

	// save access_token to file
	if err := os.WriteFile(c.getTokenFile(), []byte(c.authcode), 0o640); err != nil {
		slog.Error("failed to cache access_token", "error", err)
	}

	return c.authorize(c.authcode)
}

// getTokenFile returns the path to the access token file.
func (c *Client) getTokenFile() string {
	config := should(os.UserConfigDir())
	// we do lil trolling to any hacker accessing your computer!
	return filepath.Join(config, "drpc_"+ClientID+".bin")
}
