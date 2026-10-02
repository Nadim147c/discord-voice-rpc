// Package main provides a client for the Discord Voice RPC. It listens to the
// Discord Voice Overlay API, retrieves the voice channel state, and prints it
// to stdout as JSON.
//
// This is intended to be used for displaying a Discord overlay on any
// application, specifically for tools like Quickshell.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/go-viper/mapstructure/v2"
)

// ClientID is the client id for the discord voice rpc.
//
// NOTE: Change this if you want!
const ClientID = "207646673902501888"

// Client is the discord ipc listener and voice state manager.
type Client struct {
	rmu  sync.Mutex // guards reads
	wmu  sync.Mutex // guards writes
	sock net.Conn

	rw *bufio.ReadWriter

	ctx  context.Context
	http http.Client
	// authcode is the authorization code for discord.
	authcode string
	// loggedIn is true if the client is logged in.
	loggedIn bool
	// channelID is the ID of the voice channel.
	channelID string
	// state is the voice channel state.
	state *VoiceState
}

const (
	Handshake uint32 = 0 //	Sent by the client to initiate the connection
	Frame     uint32 = 1 // Used for all standard RPC commands and events
	Close     uint32 = 2 // Sent by either side to close the connection
	Ping      uint32 = 3 // Sent to check if the connection is alive
	Pong      uint32 = 4 // Response to a `Ping`
)

const MaxBufSize = 10 * 1024 * 1024

func (c *Client) writeJSON(op uint32, v any) error {
	p, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.writeMessage(op, p)
}

func (c *Client) writeMessage(op uint32, buf []byte) error {
	if len(buf) > MaxBufSize {
		return errors.New("discord message is too big")
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var header [8]byte
	binary.LittleEndian.PutUint32(header[:4], op)
	binary.LittleEndian.PutUint32(header[4:], uint32(len(buf))) //nolint
	_, err := c.rw.Write(header[:])
	if err != nil {
		return err
	}
	_, err = c.rw.Write(buf)
	if err != nil {
		return err
	}
	return c.rw.Flush()
}

func (c *Client) readMessage() (uint32, []byte, error) {
	c.rmu.Lock()
	defer c.rmu.Unlock()

	var header [8]byte
	_, err := io.ReadFull(c.rw, header[:])
	if err != nil {
		return 0, nil, err
	}
	op := binary.LittleEndian.Uint32(header[:4])
	size := binary.LittleEndian.Uint32(header[4:])
	if size > MaxBufSize {
		_, err := c.rw.Discard(int(size))
		if err != nil {
			return 0, nil, err
		}
		return 0, nil, errors.New("discord message is too big")
	}

	buf := make([]byte, size)
	_, err = io.ReadFull(c.rw, buf)
	if err != nil {
		return 0, nil, err
	}
	return op, buf, nil
}

// NewClient creates a new client.
func NewClient() *Client { return new(Client) }

// socketDirs returns the directories where discord creates its ipc socket.
func socketDirs() []string {
	var dirs []string
	for _, env := range []string{"XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP"} {
		if v := os.Getenv(env); v != "" {
			dirs = append(dirs, v)
		}
	}
	if v := os.Getenv("XDG_RUNTIME_DIR"); v != "" {
		dirs = append(dirs, filepath.Join(v, "app/com.discordapp.Discord"))
	}
	return append(dirs, "/tmp")
}

// dialIPC tries discord-ipc-0 .. discord-ipc-9 in every known directory and
// returns the first connection that succeeds.
func dialIPC(ctx context.Context) (net.Conn, error) {
	var d net.Dialer
	var errs []error
	for i := range 10 {
		for _, dir := range socketDirs() {
			path := filepath.Join(dir, fmt.Sprintf("discord-ipc-%d", i))
			conn, err := d.DialContext(ctx, "unix", path)
			if err == nil {
				slog.Info("Connected to discord ipc", "path", path)
				return conn, nil
			}
			errs = append(errs, err)
		}
	}
	return nil, fmt.Errorf("could not connect to discord ipc: %w", errors.Join(errs...))
}

// Listen listens to the discord ipc and handles the commands. It blocks until
// ctx is done.
func (c *Client) Listen(ctx context.Context) error {
	c.ctx = ctx
	defer func() { c.ctx = nil }()

	conn, err := dialIPC(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	context.AfterFunc(ctx, func() { conn.Close() }) //nolint

	c.sock = conn
	c.rw = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	authcode, err := os.ReadFile(c.getTokenFile())
	if err == nil {
		c.authcode = string(bytes.TrimSpace(authcode))
	}

	// discord replies with a READY dispatch frame, which handleEvent picks up.
	err = c.writeJSON(Handshake, map[string]any{"v": 1, "client_id": ClientID})
	if err != nil {
		return err
	}

	for {
		op, message, err := c.readMessage()

		// we ignore the connection err when context is done.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		switch op {
		case Ping:
			if err := c.writeMessage(Pong, message); err != nil {
				return err
			}
			continue
		case Close:
			return fmt.Errorf("discord closed the connection: %s", message)
		case Frame:
		default:
			slog.Info("ignoring ipc opcode", "op", op)
			continue
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			slog.Error("failed unmarshal message", "error", err)
			continue
		}
		slog.Debug("Message", "command", msg.Command, "event", msg.Event)

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
			os.Remove(c.getTokenFile()) //nolint
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
	case CmdGetVoiceSettings, CmdSetVoiceSettings:
		slog.Info("get voice settings", "event", msg.Data.GetString("evt"))
		return nil
	default:
		slog.Info("unknown command", "command", msg.Command)
		if debug {
			os.WriteFile("unknown-command.debug.json", should(json.Marshal(msg)), 0o640) //nolint
		}
		return nil
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
		if err := c.unsubVoice(c.channelID); err != nil {
			return err
		}
		c.state = nil
		return c.getCurrentVoiceChannel()
	case EvtError:
		slog.Error("Error event detected", "msg", msg.Data)
		return nil
	default:
		slog.Info("unknown event", "event", msg.Event)
		if debug {
			os.WriteFile("unknown-event.debug.json", should(json.Marshal(msg)), 0o640) //nolint
		}
		return nil
	}
}

// update prints the output of c.state to stdout.
func (c *Client) update() error {
	out := GetOutput(c.state)
	return json.NewEncoder(os.Stdout).Encode(out)
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
		if err := c.getCurrentVoiceChannel(); err != nil {
			slog.Error("failed to get current voice channel", "error", err)
		}
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
	return c.writeJSON(Frame, req)
}

// unsubscribe unsubscribes from an event.
func (c *Client) unsubscribe(event Event, args map[string]any) error {
	req := NewMessage(CmdUnsubscribe)
	req.Event = event
	req.Args = args
	return c.writeJSON(Frame, req)
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
	return c.writeJSON(Frame, req)
}

// authorize authorizes the client with an access token.
func (c *Client) authorize(access string) error {
	req := NewMessage(CmdAuthenticate)
	req.SetArg("access_token", access)
	slog.Info("Requesting authorization via access_token")
	return c.writeJSON(Frame, req)
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
	return c.writeJSON(Frame, req)
}

type StreamkitRequest struct {
	Code string `json:"code"`
}

// fetchAccessToken fetches an access token from Discord streamkit overlay api.
func (c *Client) fetchAccessToken(code string) error {
	slog.Info("fetching access_token from streamkit api")
	payload := StreamkitRequest{Code: code}
	buf := bytes.NewBuffer(nil)
	json.NewEncoder(buf).Encode(payload) //nolint

	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, "https://streamkit.discord.com/overlay/token", buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf := bytes.NewBuffer(nil)
		buf.ReadFrom(resp.Body) //nolint
		return fmt.Errorf("failed to get streamkit access_token: %s", buf.String())
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
	if err := os.WriteFile(c.getTokenFile(), []byte(c.authcode), 0o600); err != nil {
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
