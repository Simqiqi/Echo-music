package dj

// The Edge browser reads pages aloud through a speech service, and this
// speaks the same way: a websocket handshake carrying a proof token, the
// line as SSML, and the voice's MP3 coming back in pieces. It gives a
// natural Mandarin voice the system's own synthesizer cannot match, and
// this file falls back to nothing: the caller tries the system's voice
// when the network has none.

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/hajimehoshi/go-mp3"
)

const (
	edgeVoice = "zh-CN-XiaoxiaoNeural"
	edgePitch = "-4Hz"  // a touch below the voice's own: closer, warmer
	edgeRate  = "-4%"   // a hair slower, which suits a quiet word
	edgeDial  = 10 * time.Second
	edgeRead  = 30 * time.Second
)

var edgeSpace = regexp.MustCompile(`\s+`)

// edgeSynthesize has the service speak text, and returns the MP3 it
// made: mono, 24 kHz.
func edgeSynthesize(text string) ([]byte, error) {
	// The service refuses a couple of control ranges, and reads best
	// with the spaces evened out.
	text = edgeSpace.ReplaceAllString(strings.TrimSpace(text), " ")
	if text == "" {
		return nil, ErrNothing
	}
	url := "wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1" +
		"?TrustedClientToken=" + drmToken +
		"&Sec-MS-GEC=" + secMSGEC() +
		"&Sec-MS-GEC-Version=2-143.0.3650.75" +
		"&ConnectionId=" + edgeHex(16)
	head := [][2]string{
		{"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" +
			" (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36 Edg/143.0.0.0"},
		{"Accept-Encoding", "gzip, deflate, br, zstd"},
		{"Accept-Language", "en-US,en;q=0.9"},
		{"Pragma", "no-cache"},
		{"Cache-Control", "no-cache"},
		{"Origin", "chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold"},
		{"Cookie", "muid=" + strings.ToUpper(edgeHex(16)) + ";"},
	}
	ws, err := wsDial(url, head)
	if err != nil {
		return nil, err
	}
	defer ws.Close()
	ws.conn.SetDeadline(time.Now().Add(edgeRead))

	// The session opens with what is asked of the voice, then the line.
	cfgMsg := "X-Timestamp:" + edgeDate() + "\r\n" +
		"Content-Type:application/json; charset=utf-8\r\n" +
		"Path:speech.config\r\n\r\n" +
		`{"context":{"synthesis":{"audio":{"metadataoptions":{` +
		`"sentenceBoundaryEnabled":"false","wordBoundaryEnabled":"false"},` +
		`"outputFormat":"audio-24khz-48kbitrate-mono-mp3"}}}}` + "\r\n"
	ssml := "X-RequestId:" + edgeHex(16) + "\r\n" +
		"Content-Type:application/ssml+xml\r\n" +
		"X-Timestamp:" + edgeDate() + "Z\r\n" +
		"Path:ssml\r\n\r\n" +
		"<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='en-US'>" +
		"<voice name='" + edgeVoice + "'>" +
		"<prosody pitch='" + edgePitch + "' rate='" + edgeRate + "' volume='+0%'>" +
		edgeEscape(text) +
		"</prosody></voice></speak>"
	for _, msg := range []string{cfgMsg, ssml} {
		if err := ws.SendText(msg); err != nil {
			return nil, err
		}
	}

	// The voice comes back in binary pieces, each headed with its path;
	// a text piece ending the turn closes it.
	var mp3Data []byte
	for i := 0; i < 1000; i++ {
		buf, binaryFrame, err := ws.Receive()
		if err != nil {
			break
		}
		if !binaryFrame {
			if bytes.Contains(buf, []byte("turn.end")) {
				break
			}
			continue
		}
		if len(buf) < 2 {
			continue
		}
		hl := int(binary.BigEndian.Uint16(buf[:2]))
		if 2+hl > len(buf) {
			continue
		}
		head := string(buf[2 : 2+hl])
		body := buf[2+hl:]
		if strings.Contains(head, "Path:audio") {
			mp3Data = append(mp3Data, body...)
		} else if strings.Contains(head, "turn.end") {
			break
		}
	}
	if len(mp3Data) == 0 {
		return nil, errors.New("dj: the service said nothing")
	}
	return mp3Data, nil
}

// decodeMP3 reads an MP3 into mono samples. The decoder always hands
// back 16-bit little-endian stereo, even for a mono stream, so the two
// channels are averaged.
func decodeMP3(raw []byte) (int, []float32, error) {
	d, err := mp3.NewDecoder(bytes.NewReader(raw))
	if err != nil {
		return 0, nil, err
	}
	rate := d.SampleRate()
	if rate <= 0 {
		return 0, nil, errors.New("dj: the clip has no audio")
	}
	const ch = 2
	var out []float32
	buf := make([]byte, 8192)
	for {
		n, err := d.Read(buf)
		for i := 0; i+2*ch <= n; i += 2 * ch {
			var sum float32
			for k := 0; k < ch; k++ {
				v := int16(uint16(buf[i+k*2]) | uint16(buf[i+k*2+1])<<8)
				sum += float32(v) / 32768
			}
			out = append(out, sum/ch)
		}
		if err == io.EOF {
			return rate, out, nil
		}
		if err != nil {
			return 0, nil, err
		}
		if n == 0 {
			return 0, nil, io.ErrNoProgress
		}
	}
}

// edgeEscape keeps the line's marks from reading as markup.
func edgeEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "'", "&apos;", `"`, "&quot;")
	return r.Replace(s)
}

// edgeHex is n random bytes, as hex.
func edgeHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = byte(i * 7)
		}
	}
	return hex.EncodeToString(b)
}

// edgeDate is the timestamp the service reads, in the browser's own
// shape: "Mon Jan 02 2006 15:04:05 GMT+0000 (...)".
func edgeDate() string {
	return time.Now().UTC().Format("Mon Jan 02 2006 15:04:05 GMT+0000 (Coordinated Universal Time)")
}
