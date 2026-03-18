package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/spf13/cast"
)

type Command string

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

type Event string

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

var ErrKeyNotFound = errors.New("key not found")

type Map map[string]any

func (m *Map) ensure() {
	if *m == nil {
		*m = make(Map)
	}
}

func (m *Map) Set(k string, v any) {
	m.ensure()
	(*m)[k] = v
}

func (m Map) Has(k string) bool {
	m.ensure()
	_, ok := m[k]
	return ok
}

func (m Map) GetString(k string) string {
	return must(m.GetStringE(k))
}

func (m Map) GetStringE(k string) (string, error) {
	m.ensure()
	v, ok := m[k]
	if !ok {
		return "", ErrKeyNotFound
	}
	return cast.ToStringE(v)
}

type Request struct {
	Command Command `json:"cmd"`
	Event   Event   `json:"evt,omitempty"`
	Args    Map     `json:"args,omitempty"`
	Data    Map     `json:"data,omitempty"`
	Nonce   string  `json:"nonce"`
}

func NewRequest(cmd Command) *Request {
	r := new(Request)
	r.Command = cmd
	r.Nonce = generateRandomString()
	return r
}

func generateRandomString() string {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	const size = len(charset)
	out := [30]byte{}
	rand.Read(out[:])
	for i, b := range out {
		out[i] = charset[int(b)%size]
	}
	return string(out[:])
}

func (r *Request) SetArg(k string, v any) {
	r.Args.Set(k, v)
}

type VoiceChannel struct {
	GuildID   string       `json:"guild_id"`
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	UserLimit int64        `json:"user_limit"`
	Members   VoiceMembers `json:"voice_states"`
}

type VoiceMembers map[string]VoiceMember

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

	slices.SortFunc(slice, func(a, b VoiceMember) int {
		return strings.Compare(a.User.Username, b.User.Username)
	})

	return json.Marshal(slice)
}

type VoiceMember struct {
	Mute     bool    `json:"mute"`
	Nickname string  `json:"nick"`
	Talking  bool    `json:"talking"`
	User     User    `json:"user"`
	Status   Status  `json:"voice_state"`
	Volume   float64 `json:"volume"`
}

type User struct {
	Avatar   string `json:"avatar"`
	Bot      bool   `json:"bot"`
	Nickname string `json:"global_name"`
	ID       string `json:"id"`
	Username string `json:"username"`
}

type Decoration struct {
	Asset string `json:"asset"`
	SkuID string `json:"skuId"`
}

type status struct {
	Deaf     bool `json:"deaf"`
	Mute     bool `json:"mute"`
	SelfDeaf bool `json:"self_deaf"`
	SelfMute bool `json:"self_mute"`
	Suppress bool `json:"suppress"`
}

type Status uint32

func nthBit(n int, b bool) Status {
	if b {
		return 1 << (n - 1)
	}
	return 0
}

func (s *Status) UnmarshalJSON(data []byte) error {
	var status status
	err := json.Unmarshal(data, &status)
	if err != nil {
		return err
	}
	var n Status
	n |= nthBit(1, status.Deaf)
	n |= nthBit(2, status.Mute)
	n |= nthBit(3, status.SelfDeaf)
	n |= nthBit(4, status.SelfMute)
	n |= nthBit(5, status.Suppress)
	*s = n
	return nil
}
