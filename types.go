package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cast"
)

// should is a helper function to ignore the error.
func should[T any](v T, _ error) T { return v }

// Command is a command sent to the discord ipc.
type Command string

// String returns the string representation of the command.
func (c Command) String() string {
	return string(c)
}

const (
	CmdAuthorize               Command = "AUTHORIZE"
	CmdAuthenticate            Command = "AUTHENTICATE"
	CmdGetSelectedVoiceChannel Command = "GET_SELECTED_VOICE_CHANNEL"
	CmdGetVoiceSettings        Command = "GET_VOICE_SETTINGS"
	CmdSetVoiceSettings        Command = "SET_VOICE_SETTINGS"
	CmdSubscribe               Command = "SUBSCRIBE"
	CmdUnsubscribe             Command = "UNSUBSCRIBE"
	CmdDispatch                Command = "DISPATCH"
)

// Event is an event sent by the discord ipc.
type Event string

// String returns the string representation of the event.
func (e Event) String() string {
	return string(e)
}

const (
	EvtReady              Event = "READY"
	EvtError              Event = "ERROR"
	EvtVoiceStateCreate   Event = "VOICE_STATE_CREATE"
	EvtVoiceStateUpdate   Event = "VOICE_STATE_UPDATE"
	EvtVoiceStateDelete   Event = "VOICE_STATE_DELETE"
	EvtSpeakingStart      Event = "SPEAKING_START"
	EvtSpeakingStop       Event = "SPEAKING_STOP"
	EvtVoiceChannelSelect Event = "VOICE_CHANNEL_SELECT"
)

// ErrKeyNotFound is returned when a key is not found in the map.
var ErrKeyNotFound = errors.New("key not found")

// Map is a map of strings to any. It is used to store the arguments and data of
// a message.
type Map map[string]any

// ensure ensures that the map is initialized.
func (m *Map) ensure() {
	if *m == nil {
		*m = make(Map)
	}
}

// Set sets the value of the key.
func (m *Map) Set(k string, v any) {
	m.ensure()
	(*m)[k] = v
}

// Has checks if the key exists in the map.
func (m Map) Has(k string) bool {
	m.ensure()
	_, ok := m[k]
	return ok
}

// GetString is like GetString but ignores the error and returns an empty string
// if the key is not found.
func (m Map) GetString(k string) string {
	return should(m.GetStringE(k))
}

// GetStringE returns the value of the key as a string. It returns an error if
// the key is not found or failed to convert the data to string.
func (m Map) GetStringE(k string) (string, error) {
	m.ensure()
	v, ok := m[k]
	if !ok {
		return "", ErrKeyNotFound
	}
	return cast.ToStringE(v)
}

// Message is a message sent to the discord ipc.
type Message struct {
	Command Command `json:"cmd"`
	Event   Event   `json:"evt,omitempty"`
	Args    Map     `json:"args,omitempty"`
	Data    Map     `json:"data,omitempty"`
	Nonce   string  `json:"nonce"`
}

// NewMessage creates a new request with the given command.
func NewMessage(cmd Command) *Message {
	return &Message{
		Command: cmd,
		Nonce:   generateRandomString(),
	}
}

// generateRandomString generates a random string of the given length with a-z
// charset.
func generateRandomString() string {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	const size = len(charset)
	out := [30]byte{}
	rand.Read(out[:])
	for i, b := range out {
		out[i] = charset[int(b)%size] // normalize into a-z range
	}
	return string(out[:])
}

// SetArg sets the value of the key in the arguments map.
func (r *Message) SetArg(k string, v any) {
	r.Args.Set(k, v)
}

// SetData sets the value of the key in the data map.
func (r *Message) SetData(k string, v any) {
	r.Data.Set(k, v)
}

// VoiceState is the voice channel state. It holds users and their voice states.
type VoiceState struct {
	GuildID   string       `json:"guild_id"`
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	UserLimit int64        `json:"user_limit"`
	Members   VoiceMembers `json:"voice_states"`
}

// VoiceMembers is a map of users and their voice states. It implemets
// json.Unmarshaler and json.Marshaler so the it can be encoded and decoded
// as/from json array.
type VoiceMembers map[string]VoiceMember

// UnmarshalJSON unmarshals the voice members from JSON.
func (vm *VoiceMembers) UnmarshalJSON(data []byte) error {
	var slice []VoiceMember
	if err := json.Unmarshal(data, &slice); err != nil {
		return err
	}
	*vm = make(VoiceMembers, len(slice))
	for _, member := range slice {
		(*vm)[member.User.ID] = member
	}
	return nil
}

func (vm VoiceMembers) MarshalJSON() ([]byte, error) {
	slice := make([]VoiceMember, 0, len(vm))
	for _, member := range vm {
		slice = append(slice, member)
	}

	// sort users by username
	slices.SortFunc(slice, func(a, b VoiceMember) int {
		return strings.Compare(a.User.Username, b.User.Username)
	})

	return json.Marshal(slice)
}

// VoiceMember is a  member of voice channel.
type VoiceMember struct {
	Mute     bool    `json:"mute"`
	Nickname string  `json:"nick"`
	Talking  bool    `json:"talking"`
	User     User    `json:"user"`
	Status   Status  `json:"voice_state"`
	Volume   float64 `json:"volume"`
}

// User is a discord user.
type User struct {
	Avatar   string `json:"avatar"`
	Nickname string `json:"global_name"`
	Bot      bool   `json:"bot"`
	ID       string `json:"id"`
	Username string `json:"username"`
}

// AvatarURL returns the avatar URL of the user.
func (u User) AvatarURL() string {
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", u.ID, u.Avatar)
}

// Status is a voice status of a VoiceMember.
type Status struct {
	Deaf     bool `json:"deaf"`
	Mute     bool `json:"mute"`
	SelfDeaf bool `json:"self_deaf"`
	SelfMute bool `json:"self_mute"`
	Suppress bool `json:"suppress"`
}

func toggleBit(b bool, i uint8) uint8 {
	if b {
		return i
	}
	return 0
}

func (s Status) Encode() uint8 {
	var out uint8
	out |= toggleBit(s.Mute, 1<<0)     // 1
	out |= toggleBit(s.SelfMute, 1<<1) // 2
	out |= toggleBit(s.Deaf, 1<<2)     // 4
	out |= toggleBit(s.SelfDeaf, 1<<3) // 8
	out |= toggleBit(s.Suppress, 1<<4) // 16
	return out
}

// Output is the output of the discord voice rpc used for printing output.
type Output struct {
	GuildID     string         `json:"guildId"`
	ChannelID   string         `json:"channelId"`
	ChannelName string         `json:"channelName"`
	UserLimit   int64          `json:"userLimit"`
	Members     []OutputMember `json:"members"`
}

// OutputMember is a member of the voice channel used for printing output.
type OutputMember struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Nickname   string `json:"nickname"`
	ServerName string `json:"serverName"`
	Avatar     string `json:"avatar"`
	AvatarURL  string `json:"avatarURL"`
	IsTalking  bool   `json:"isTalking"`
	IsBot      bool   `json:"isBot"`
	Status     uint8  `json:"status"`
}

// GetOutput converts a VoiceState to an Output.
func GetOutput(vs *VoiceState) *Output {
	if vs == nil {
		return nil
	}
	members := make([]OutputMember, 0, len(vs.Members))
	for _, member := range vs.Members {
		members = append(members, OutputMember{
			ID:         member.User.ID,
			Username:   member.User.Username,
			Nickname:   member.Nickname,
			ServerName: member.User.Nickname,
			Avatar:     member.User.Avatar,
			AvatarURL:  member.User.AvatarURL(),
			IsTalking:  member.Talking,
			IsBot:      member.User.Bot,
			Status:     member.Status.Encode(),
		})
	}
	return &Output{
		GuildID:     vs.GuildID,
		ChannelID:   vs.ID,
		ChannelName: vs.Name,
		UserLimit:   vs.UserLimit,
		Members:     members,
	}
}
