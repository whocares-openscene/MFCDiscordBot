// FCS websocket wire format: parses raw frame bytes into typed application
// messages and builds outbound frames for the requests we send. The
// dispatcher does a type switch on the parsed result.
package mfc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	types "mfcbot/internal"
	"mfcbot/internal/logging"
)

// FCS frame type constants and protocol values.
const (
	mfcFCTypeNull           = 0
	mfcFCTypeLogin          = 1
	mfcFCTypeUsernameLookup = 10
	mfcFCTypeManageList     = 14
	mfcFCTypeSessionState   = 20
	mfcFCTypeRoomData       = 44
	mfcFCTypeExtData        = 81

	mfcFCLCams         = 21
	mfcFCWOptRedisJSON = 256
	mfcFCVideoOffline  = 127

	mfcFrameLenDigits = 6 // width of FCS frame length prefix
)

// FCS handshake/keepalive constants used by the encoders below.
const (
	mfcWSHandshakeFrame = "fcsws_20180422\n\x00"
	mfcNullFrame        = "0 0 0 0 0\n\x00"

	mfcLoginVersion = "20080910"
	mfcLoginCreds   = "guest:guest"
)

// encodeLogin builds an FCTYPE.LOGIN frame for the guest credentials we
// connect with.
func encodeLogin() string {
	return fmt.Sprintf("%d 0 0 %s 0 %s\n\x00",
		mfcFCTypeLogin, mfcLoginVersion, mfcLoginCreds)
}

// encodeUsernameLookupByUID builds an FCTYPE.USERNAMELOOKUP frame whose
// payload is a numeric uid.
func encodeUsernameLookupByUID(qid int64, uid int) string {
	return fmt.Sprintf("%d 0 0 %d %d\n\x00", mfcFCTypeUsernameLookup, qid, uid)
}

// encodeUsernameLookupByName builds an FCTYPE.USERNAMELOOKUP frame whose
// payload is a username.
func encodeUsernameLookupByName(qid int64, name string) string {
	return fmt.Sprintf("%d 0 0 %d 0 %s\n\x00",
		mfcFCTypeUsernameLookup, qid, url.QueryEscape(name))
}

// looksLikeJSONObject reports whether b's first non-whitespace byte is `{`.
func looksLikeJSONObject(b []byte) bool {
	for _, c := range b {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		return c == '{'
	}
	return false
}

// frame is one parsed FCS frame body, before decoding into a typed message.
type frame struct {
	fcType, from, to, arg1, arg2 int
	payload                      []byte
}

// parseFrame parses one FCS frame body.
func parseFrame(body []byte) (frame, bool) {
	parts := bytes.SplitN(body, []byte(" "), 6)
	if len(parts) < 5 {
		return frame{}, false
	}
	nums := [5]int{}
	for i := range 5 {
		n, err := strconv.Atoi(string(parts[i]))
		if err != nil {
			return frame{}, false
		}
		nums[i] = n
	}
	f := frame{
		fcType: nums[0], from: nums[1], to: nums[2],
		arg1: nums[3], arg2: nums[4],
	}
	if len(parts) == 6 && len(parts[5]) > 0 {
		decoded, err := url.QueryUnescape(string(parts[5]))
		if err != nil {
			return frame{}, false
		}
		f.payload = []byte(decoded)
	}
	return f, true
}

// frameWalkError describes a parser-desync failure inside walkFrames.
type frameWalkError struct {
	failFrame  int
	failOffset int
	origLen    int
	kind       string
}

func (e *frameWalkError) Error() string {
	return fmt.Sprintf("%s at byte %d of %d (frame %d)",
		e.kind, e.failOffset, e.origLen, e.failFrame)
}

// walkFrames extracts each length-prefixed FCS frame from data and invokes
// visit on it. It stops early when visit returns true.
func walkFrames(data []byte, visit func(f frame) (stop bool)) (consumed int, err error) {
	origLen := len(data)
	framesWalked := 0
	for len(data) >= mfcFrameLenDigits {
		offset := origLen - len(data)
		bodyLen, atoErr := strconv.Atoi(string(data[:mfcFrameLenDigits]))
		if atoErr != nil || bodyLen < 0 {
			return offset, &frameWalkError{
				failFrame:  framesWalked,
				failOffset: offset,
				origLen:    origLen,
				kind:       "invalid frame prefix",
			}
		}
		if bodyLen > len(data)-mfcFrameLenDigits {
			return offset, nil
		}
		body := data[mfcFrameLenDigits : mfcFrameLenDigits+bodyLen]
		data = data[mfcFrameLenDigits+bodyLen:]
		f, ok := parseFrame(body)
		if !ok {
			return offset, &frameWalkError{
				failFrame:  framesWalked,
				failOffset: offset,
				origLen:    origLen,
				kind:       fmt.Sprintf("malformed frame body, %d bytes", len(body)),
			}
		}
		if visit(f) {
			return origLen - len(data), nil
		}
		framesWalked++
	}
	return origLen - len(data), nil
}

// mfcMessage is one typed application-level FCS message.
type mfcMessage interface {
	isMFCMessage()
}

type bulkRefMsg struct {
	ext *mfcExtData
}

func (*bulkRefMsg) isMFCMessage() {}

type sessionStateMsg struct {
	update sessionUpdate
}

func (*sessionStateMsg) isMFCMessage() {}

type lookupResponseMsg struct {
	qid     int
	payload []byte
	update  sessionUpdate
}

func (*lookupResponseMsg) isMFCMessage() {}

type roomDataMsg struct {
	counts map[int]int
}

func (*roomDataMsg) isMFCMessage() {}

// decodeFrame turns a raw frame into a typed message.
func decodeFrame(f frame) (mfcMessage, error) {
	switch f.fcType {
	case mfcFCTypeSessionState:
		u, err := parseSessionState(f.payload)
		if err != nil {
			return nil, fmt.Errorf("%w; payload head = %s", err, dumpBytes(f.payload))
		}
		return &sessionStateMsg{update: u}, nil
	case mfcFCTypeUsernameLookup:
		msg := &lookupResponseMsg{qid: f.arg1, payload: f.payload}
		if looksLikeJSONObject(f.payload) {
			u, err := parseSessionState(f.payload)
			if err != nil {
				return nil, fmt.Errorf("lookup response (qid=%d), %w; payload head = %s",
					f.arg1, err, dumpBytes(f.payload))
			}
			msg.update = u
		}
		return msg, nil
	case mfcFCTypeRoomData:
		counts, err := parseRoomData(f.payload)
		if err != nil {
			return nil, fmt.Errorf("%w; payload head = %s", err, dumpBytes(f.payload))
		}
		return &roomDataMsg{counts: counts}, nil
	case mfcFCTypeExtData:
		return decodeBulkRef(f)
	}
	return nil, nil
}

// decodeBulkRef returns a bulkRefMsg if f is a MANAGELIST/CAMS bulk-dump
// pointer.
func decodeBulkRef(f frame) (mfcMessage, error) {
	if f.arg2 != mfcFCWOptRedisJSON {
		return nil, nil
	}
	var ext mfcExtData
	if err := json.Unmarshal(f.payload, &ext); err != nil {
		return nil, fmt.Errorf("extdata, %w; payload head = %s", err, dumpBytes(f.payload))
	}
	if ext.Msg.Type != mfcFCTypeManageList || ext.Msg.Arg2 != mfcFCLCams {
		return nil, nil
	}
	return &bulkRefMsg{ext: &ext}, nil
}

type mfcExtDataMsg struct {
	Type int `json:"type"`
	From int `json:"from"`
	To   int `json:"to"`
	Arg1 int `json:"arg1"`
	Arg2 int `json:"arg2"`
}

type mfcExtData struct {
	Msg     mfcExtDataMsg `json:"msg"`
	MsgLen  int           `json:"msglen"`
	Opts    int           `json:"opts"`
	RespKey int           `json:"respkey"`
	Serv    int           `json:"serv"`
	Type    int           `json:"type"`
}

type bulkRow struct {
	name    string
	uid     int
	vs      int
	rc      int
	topic   string
	camserv int
}

type bulk []bulkRow

type sessionUpdateMeta struct {
	RC    *int    `json:"rc"`
	Topic *string `json:"topic"`
}

type sessionUpdateUser struct {
	Camserv *int `json:"camserv"`
}

type sessionUpdate struct {
	Name *string            `json:"nm"`
	UID  *int               `json:"uid"`
	VS   *int               `json:"vs"`
	M    *sessionUpdateMeta `json:"m"`
	U    *sessionUpdateUser `json:"u"`
}

func (u sessionUpdate) rc() *int {
	if u.M == nil {
		return nil
	}
	return u.M.RC
}

func (u sessionUpdate) topic() *string {
	if u.M == nil {
		return nil
	}
	return u.M.Topic
}

func (u sessionUpdate) camserv() *int {
	if u.U == nil {
		return nil
	}
	return u.U.Camserv
}

func parseSessionState(payload []byte) (sessionUpdate, error) {
	logging.Ltrace("sessionstate raw: %s", payload)
	var u sessionUpdate
	if err := json.Unmarshal(payload, &u); err != nil {
		return sessionUpdate{}, fmt.Errorf("sessionstate, %w", err)
	}
	return u, nil
}

func parseRoomData(payload []byte) (map[int]int, error) {
	var raw map[string]int
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("roomdata, %w", err)
	}
	parsed := make(map[int]int, len(raw))
	for k, v := range raw {
		uid, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		parsed[uid] = v
	}
	return parsed, nil
}

func parseBulkRData(rdata []any) (bulk, error) {
	expectedFields := []string{"uid", "nm", "vs", "m.rc", "m.topic", "u.camserv"}
	if len(rdata) == 0 {
		return nil, nil
	}
	schema, ok := rdata[0].([]any)
	if !ok {
		return nil, errors.New("missing schema")
	}
	paths := make([]string, 0, len(schema))
	for _, item := range schema {
		switch v := item.(type) {
		case string:
			paths = append(paths, v)
		case map[string]any:
			if len(v) != 1 {
				return nil, fmt.Errorf("bulk schema: nested item has %d keys, expected 1", len(v))
			}
			for k, sub := range v {
				subs, ok := sub.([]any)
				if !ok {
					continue
				}
				for _, s := range subs {
					if name, ok := s.(string); ok {
						paths = append(paths, k+"."+name)
					}
				}
			}
		}
	}
	idx := map[string]int{}
	for i, p := range paths {
		idx[p] = i
	}
	if _, ok := idx["uid"]; !ok {
		return nil, fmt.Errorf("bulk schema missing uid (have %v)", paths)
	}
	var missing []string
	for _, exp := range expectedFields {
		if _, ok := idx[exp]; !ok {
			missing = append(missing, exp)
		}
	}
	if len(missing) > 0 {
		logging.Lerr("bulk schema missing expected fields %v (have %v)", missing, paths)
	}
	get := func(row []any, key string) any {
		i, ok := idx[key]
		if !ok || i >= len(row) {
			return nil
		}
		return row[i]
	}
	toInt := func(v any) int {
		if n, ok := v.(float64); ok {
			return int(n)
		}
		return 0
	}
	toStr := func(v any) string {
		if s, ok := v.(string); ok {
			return s
		}
		return ""
	}
	out := make(bulk, 0, len(rdata)-1)
	for _, rec := range rdata[1:] {
		row, ok := rec.([]any)
		if !ok {
			continue
		}
		out = append(out, bulkRow{
			name:    toStr(get(row, "nm")),
			uid:     toInt(get(row, "uid")),
			vs:      toInt(get(row, "vs")),
			rc:      toInt(get(row, "m.rc")),
			topic:   toStr(get(row, "m.topic")),
			camserv: toInt(get(row, "u.camserv")),
		})
	}
	return out, nil
}

// decodeMFCTopic URL-decodes an MFC topic string.
func decodeMFCTopic(s string) string {
	if s == "" {
		return s
	}
	if decoded, err := url.QueryUnescape(s); err == nil {
		return decoded
	}
	return s
}

// mfcStateName returns a fine-grained human-readable name for an MFC video
// state.
func mfcStateName(vs int) string {
	switch vs {
	case 0:
		return "free"
	case 2:
		return "away"
	case 12:
		return "private (tx)"
	case 13:
		return "group (tx)"
	case 14:
		return "club (tx)"
	case 90:
		return "idle"
	case 91:
		return "private (rx)"
	case 93:
		return "group (rx)"
	case 94:
		return "club (rx)"
	case mfcFCVideoOffline:
		return "offline"
	}
	return fmt.Sprintf("vs=%d", vs)
}

// mfcShowKind maps an MFC video state to a ShowKind.
func mfcShowKind(vs int) types.ShowKind {
	switch vs {
	case 0, 90:
		return types.ShowPublic
	case 12, 91:
		return types.ShowPrivate
	case 13, 93:
		return types.ShowGroup
	case 14, 94:
		return types.ShowTicket
	case 2:
		return types.ShowAway
	}
	return types.ShowUnknown
}

// dumpHeadBytes bounds the diagnostic-dump head.
const dumpHeadBytes = 256

func dumpBytes(b []byte) string {
	if len(b) > dumpHeadBytes {
		b = b[:dumpHeadBytes]
	}
	var sb bytes.Buffer
	for _, c := range b {
		switch {
		case c == '\\':
			sb.WriteString(`\\`)
		case c == '\n':
			sb.WriteString(`\n`)
		case c == '\r':
			sb.WriteString(`\r`)
		case c == '\t':
			sb.WriteString(`\t`)
		case c >= 0x20 && c < 0x7f:
			sb.WriteByte(c)
		default:
			fmt.Fprintf(&sb, `\x%02x`, c)
		}
	}
	return sb.String()
}
